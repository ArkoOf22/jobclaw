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
| Disk | 20 GB, ~45% used |

The instance name predates the project's rename from OpenClaw to JobClaw.

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

- **`/home/openclaw/jobspy-mcp`** — the JobSpy MCP server with its own `.venv`. Currently **not
  running**; nothing listens on port 8000. JobSpy discovery cannot work until it is started.
  This is the missing piece behind "MCP wiring incomplete."
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
