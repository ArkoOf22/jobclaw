# Runtime Environment

JobClaw is developed and run on a dedicated EC2 host. The local macOS checkout is for editing only;
it cannot build (Go 1.25.5 vs the required 1.26.5).

## The build host

| | |
| :--- | :--- |
| Name | `openclaw-prod` |
| Instance | `i-07db177939730f8ec` |
| Region | `ap-south-1` |
| Type | `t3.small` |
| OS | Ubuntu 22.04.5 LTS |
| Go | 1.26.5 at `/usr/local/go` (not on root's PATH) |
| Project path | `/home/openclaw/jobclaw` |
| Owning user | `openclaw` |
| Disk | 20 GB, ~66% used |
| RAM | 1.9 GB usable, plus 2 GB swap at `/swapfile` |

The instance name predates the project's rename from OpenClaw to JobClaw.

## Memory: 2 GB of swap, and why

The box has under 2 GB of RAM and `openclaw-gateway` was OOM-killed twice
(2026-09-06 and 2026-09-19) with no swap to fall back on. `dmesg -T | grep -i
"killed process"` shows both. The cause is the agent's own replayed context, not
JobClaw.

A 2 GB swap file was added on 2026-09-19, persisted in `/etc/fstab`
(`/swapfile none swap sw 0 0`, backed up as `/etc/fstab.bak-*`) with
`vm.swappiness=10` in `/etc/sysctl.d/99-jobclaw-swappiness.conf`.

Swappiness is deliberately low: swap here is an emergency cushion that turns a
kill into slowness, not a routine tier the kernel should page into under normal
load. `findmnt --verify --fstab` reports 0 errors; it warns that the source is a
regular file, which is what a swap file is, so that warning is expected.

This does not make the host roomy. It makes a memory spike survivable. The real
fix is trimming what the agent replays on every poll.

## Access

Connect over **SSM Session Manager**, not the public internet. Inbound SSH in `openclaw-sg`
(`sg-036c4ebe4a6bcec76`) is pinned to a single stale IP (`106.222.200.176/32`), so direct SSH from
anywhere else times out. SSM sidesteps this entirely and is CloudTrail-audited.

An SSH-over-SSM alias is configured in `~/.ssh/config` on the Mac:

```bash
ssh jobclaw                                  # interactive shell, lands as `ubuntu`
ssh jobclaw 'sudo -u openclaw bash -lc "..."'  # run as the project owner
```

Round trip is roughly 5 seconds. Only the `ubuntu` user has the `openclaw-key` public key, so always
land as `ubuntu` and step into `openclaw` with `sudo -u openclaw`.

Standard command shape — the login shell does not put Go on PATH:

```bash
ssh jobclaw 'sudo -u openclaw bash -lc "cd /home/openclaw/jobclaw && export PATH=\$PATH:/usr/local/go/bin && go test ./..."'
```

`aws ssm send-command` also works but is far slower and clunkier. Prefer the SSH alias.

Do not widen the security group to restore plain SSH unless explicitly asked. It is a security
control, and SSM already provides access.

## Also on the host

- **`/home/openclaw/jobspy-mcp`** — the JobSpy MCP server, now running as a systemd
  service (`jobspy-mcp.service`, enabled at boot) on `127.0.0.1:8000`, which is
  `defaultJobSpyURL`. The unit is version-controlled at `deploy/jobspy-mcp.service`.

  Manage it with `sudo systemctl {status,restart} jobspy-mcp`; logs go to
  `/home/openclaw/jobspy-mcp/logs/jobspy.log`.

  Scrapes LinkedIn, Indeed, Glassdoor, ZipRecruiter, Google Jobs, Bayt, Naukri, and
  BDJobs. Verified working from this instance despite the datacenter IP, returning
  real Bengaluru backend roles. Success rates may degrade over time, since job
  boards actively block datacenter ranges; the upstream project suggests proxies if
  that happens.

  The HTTP endpoint is **unauthenticated** unless `JOBSPY_HTTP_TOKEN` is set. It is
  bound to loopback, so keep it there.

  Note the JobSpy server has **two independent consumers** and they are unrelated.
  JobClaw's own Go client (`internal/discovery/jobspy/client.go`, endpoint
  `http://127.0.0.1:8000/mcp`) is what fills the database. The `job-search-mcp`
  OpenClaw skill is a separate front door for ad-hoc chat searches and stores
  nothing. JobClaw does not go through that skill, so the skill's presence or
  absence has no effect on discovery.

- **`jobclaw-metrics.service`** — `bin/jobclaw metrics` on `127.0.0.1:9090`,
  enabled at boot, serving `/metrics` in Prometheus text format and `/healthz`.
  Unit is version-controlled at `deploy/jobclaw-metrics.service`, capped at
  `MemoryMax=192M`, measured resident size ~3 MB.

  **Unauthenticated**, like JobSpy. Keep it on loopback or a Tailscale address;
  never `0.0.0.0`.

- **`alloy.service`** — Grafana Alloy v1.19.2, enabled at boot. Scrapes the metrics
  endpoint (60s) and host metrics (30s), then remote-writes to Grafana Cloud in
  `ap-south-1`. Capped at `MemoryMax=256M`, measured ~52 MB.

  Config at `/etc/alloy/config.alloy` (from `deploy/alloy/config.alloy`); systemd
  drop-in at `/etc/systemd/system/alloy.service.d/override.conf` (from
  `deploy/alloy/alloy-override.conf`). Runs as user `alloy`.

  Credentials live in `/etc/alloy/grafana-cloud.env`, root-only mode 600 and
  **untracked**. systemd reads it as root before dropping privileges, so 600 is
  correct despite the service running as `alloy`. The `EnvironmentFile` has no
  leading `-` on purpose: missing credentials must fail loudly rather than start
  and silently push nothing.

  Alloy's own diagnostics are on `127.0.0.1:12345`. To confirm delivery:

  ```bash
  curl -s http://127.0.0.1:12345/metrics | grep remote_storage_samples
  ```

  `samples_failed_total` should stay at 0. Setup guide: `docs/grafana-cloud-setup.md`.
  Because host metrics include `node_vmstat_oom_kill` and PSI memory pressure, the
  next OOM kill is now visible as a graph rather than found later in `dmesg`.

## The agent's context bundle is the real cost and memory driver

Not JobClaw. `OPENROUTER_API_KEY` is unset and no resumes have been generated, so
JobClaw's own model spend is nil. The spend and the memory pressure both come from
the OpenClaw agent's per-request context.

Measured on 2026-09-19 across 200 session trajectories and 478 model calls:

| Prompt tokens per call | |
| :--- | :--- |
| median | 20,022 |
| p75 | 85,633 |
| max | 511,411 |

Bimodal: routine calls sit near 20k, with a heavy tail from long working sessions.

Composition of a large request, from `data.systemPrompt`, `data.tools` and
`data.messages` in the trajectory files:

| Component | Share |
| :--- | :--- |
| conversation messages | 54% |
| system prompt | 25% |
| tool definitions | 20% |

Where to measure it: `~/.openclaw/agents/main/sessions/*.trajectory.jsonl`. Real
token counts are at `data.usage.{input,output,total}`; cache activity is at
`data.promptCache.lastCallUsage.{cacheRead,cacheWrite}`. These files contain
personal conversation, so measure sizes rather than printing content.

Three things that are **not** true, each of which cost time to disprove:

- **There is no poll timer.** The agent is event-driven. Calls are triggered by
  Telegram messages and by `hooks.gmail`, which watches `label = INBOX` with
  `includeBody = true` and `maxBytes = 20000`. Every inbox email wakes the agent
  and runs a full-bundle call, so inbox volume is what drives call count.
- **Skill file size does not matter per call.** Only skill *descriptions* ship in
  the system prompt; bodies load on activation. Verified by searching the captured
  system prompt for distinctive strings from each `SKILL.md`. Shortening a skill
  file saves nothing.
- **Prompt caching is off, and it should stay off.** Zero cache reads, zero cache
  writes. The system prompt plus tool definitions are 46% of every request and
  byte-identical each time, so this looks like the textbook caching case. It is
  not worth pursuing, and the reason is pricing rather than plumbing. See below.

### Settled: prompt caching is not worth enabling

Investigated on 2026-09-19 with `diagnostics.cacheTrace` enabled temporarily.
Do not re-litigate this without new pricing.

OpenClaw is already doing its part. The trace shows it tracks
`systemPromptDigest` and `toolDigest`, and the `cache:result` stage reports
`note = "stable cache inputs"` — it correctly recognises the prompt prefix is
reusable. What it will not do is send a cache key, because the model entry for
`openrouter/qwen/qwen3.5-flash-02-23` carries `cost.cacheRead = 0`,
`cost.cacheWrite = 0` and no `supportsPromptCacheKey` in `compat`. The call goes
out over `modelApi = openai-completions`, so Anthropic-style `cache_control`
markers do not apply to this path at all.

The pricing is what closes it. Per OpenRouter's own model API:

| model | in $/M | out $/M | cache read $/M |
| :--- | ---: | ---: | ---: |
| `qwen/qwen3.5-flash-02-23` (current) | 0.065 | 0.26 | none offered |
| `google/gemini-2.5-flash` | 0.30 | 2.50 | 0.03 |
| `google/gemini-2.5-flash-lite` | 0.10 | 0.40 | 0.01 |

For a routine 20,000-prompt, 400-completion call:

- qwen, no caching: **$0.001404**
- gemini-2.5-flash, cache hitting on all 46%: $0.004516 — **3.2x more expensive**
- gemini-2.5-flash-lite, cache hitting: $0.001332 — 5% cheaper, but $0.002160 on
  a cache miss, which is 54% *more* expensive

So the best case for switching is a 5% saving that depends on every call hitting
a warm cache, against a 54% penalty when it does not. Output pricing is what
dominates: gemini-2.5-flash bills output at $2.50/M against qwen's $0.26/M,
nearly ten times, and no discount on input prefix can recover that.

The current model is simply cheaper without caching than the cache-capable
alternatives are with it. This is the same trap as `tech.md`'s note about not
chasing the OpenRouter discount list: the headline feature is not the price.

Turn the trace back on the same way if pricing changes:

```bash
openclaw config patch --file /tmp/ct.json5   # diagnostics.cacheTrace.enabled=true
sudo systemctl restart openclaw-gateway
```

Set `includeMessages`, `includePrompt` and `includeSystem` to false when doing so.
Those default to true and would write conversation content to
`~/.openclaw/logs/cache-trace.jsonl`. Disable it again afterwards.

### Applied: unused media tools denied

`tools.profile` is `coding`, which loaded 26 tools including `video_generate`,
`image_generate` and `music_generate` — descriptions of video and music generation
sent to a job-search bot on every call.

Denied globally via `tools.deny` rather than changing the profile, because
switching to `messaging` risked dropping `exec`, and the JobClaw skill is entirely
shell-based through `scripts/jobclaw-agent`. A denylist overrides the profile
without touching anything else.

Measured effect: 26 tools to 23, and the tool block from 26,367 to 19,193 bytes.
`skill_workshop` was deliberately kept — it is what created the Gmail skill, per
`~/.openclaw/skill-workshop/proposals.json`.

Config backed up first as `~/.openclaw/openclaw.json.bak-pre-tooldeny-*`. Verify
with `openclaw config get tools.deny`. To measure a change without waiting for a
real event:

```bash
openclaw agent --session-key verify-1 -m "Reply with the single word: ok"
```

Omitting `--deliver` keeps the reply off Telegram.
- **Tailscale** (`100.107.167.115`) — an alternative network path if SSM is ever unavailable.
- `openclaw-gateway` on `127.0.0.1:18789` and `gog` on `127.0.0.1:8788` — unrelated tooling.

## Git

The host's remote is SSH (`git@github.com:ArkoOf22/jobclaw.git`); the Mac's is HTTPS. Both point at
the same repo.

Git refuses to operate on `/home/openclaw/jobclaw` as root ("dubious ownership"). Always run git
through `sudo -u openclaw`, never as root, and do not add a global `safe.directory` exception.

The GitHub repo is owned by `ArkoOf22`. The Mac's active `gh` account is `Arko-Twid`, which has push
but **not admin**. Repo settings changes need an account switch to `ArkoOf22`.

## Live state

The database at `data/jobclaw.db` holds real work, not fixtures. Treat it as production data:

- 20 discovered jobs, all Stripe via the Greenhouse source.
- Job 7 was approved and has a generated tailored resume at
  `data/applications/7/resume/tailored_resume.txt`.
- Master resume at `data/resume/master_resume.txt`.

Commands that mutate this state include `approve`, `reject`, `application <jobID>` (which calls
`CreateForApprovedJob`), `prepare`, and `submit`. Back up the DB before destructive experiments, and
prefer a scratch `JOBCLAW_DB_PATH` for testing.

## Credentials

`OPENROUTER_API_KEY` **is** present in `/home/openclaw/jobclaw/.env` and resume
tailoring works. Verified 2026-09-19: the key is set, and tailored resumes exist
for applications 7, 473 and 701, with 701 compiled through LaTeX to a PDF.

This section previously said the key was missing and every LLM-backed path
failed. That was true when written and is no longer. If `resume`, `application`,
`prepare` or questionnaire fallback fail with
`resume LLM API key environment variable "OPENROUTER_API_KEY" is not set`, the
`.env` was not sourced rather than the key being absent — the scheduled units
source it explicitly, so check that first.

`.env` also holds `JOBCLAW_SHEET_ID`, `JOBCLAW_SHEET_ACCOUNT` and
`GOG_KEYRING_PASSWORD`. The last is not optional: `gog` cannot prompt for a
keyring password from a timer.

## The real bottleneck is conversion, not capability

Worth stating plainly because every remaining engineering instinct points at the
wrong end of the pipeline. As of 2026-09-19:

| Stage | Count |
| :--- | ---: |
| Jobs discovered | 2,161 |
| Shortlisted | 639 |
| Awaiting a human decision | 2,150 |
| Applications created | 2 |
| Jobs marked applied | 9 |

Discovery, scoring, resume tailoring, questionnaire ingestion and readiness
validation all work. Almost nothing flows through them, because the approval gate
is deliberately human and 2,150 decisions are queued behind one person.

This is not a defect to fix in code, and adding more discovery sources or more
scoring accuracy will not move it — both make the queue longer. The useful work
is at the far end: getting a handful of the top-scoring jobs through approval,
resume, questionnaire and submission, and recording outcomes with
`jobclaw mark`. Until outcomes are recorded there is also no signal to calibrate
scoring against, so the loop cannot close on itself.
