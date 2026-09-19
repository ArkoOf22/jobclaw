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

## Missing credential

There is **no `.env`** on the host and `OPENROUTER_API_KEY` is not in any shell profile. Every
LLM-backed path fails without it: resume tailoring, questionnaire answer fallback, and the
`application`, `prepare`, and `resume` subcommands all abort with
`resume LLM API key environment variable "OPENROUTER_API_KEY" is not set`.

This must be supplied at `/home/openclaw/jobclaw/.env` before any end-to-end run.
