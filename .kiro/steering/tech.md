# Tech Stack & Commands

## Stack

- **Go** — `go.mod` declares `go 1.26.5`, module name `jobclaw`.
- **SQLite** via `modernc.org/sqlite` (pure Go, no cgo).
- **LLM** via OpenRouter, behind interfaces in `internal/llm`. Model is swappable through config,
  never hardcoded into workflow logic.
- **MCP** via `github.com/modelcontextprotocol/go-sdk` — used for JobSpy discovery.
- **Config** via `gopkg.in/yaml.v3`.

Direct dependencies are few and deliberate. Prefer the standard library before adding anything new.

## Where the code runs

**Build and run on EC2, not locally.** See `environment.md` for access details. The EC2 host has
Go 1.26.5, which is what `go.mod` requires.

The local macOS checkout has Go 1.25.5 with `GOTOOLCHAIN=local`, so builds fail there with
`go.mod requires go >= 1.26.5`. That is expected — the Mac is for editing, EC2 is for building.
Do not "fix" it by downgrading `go.mod`.

## Commands

Run these on the EC2 host, from `/home/openclaw/jobclaw` as the `openclaw` user:

```bash
go build ./...
go vet ./...
go test ./...                    # full suite
go test ./internal/application   # single package
go test -run TestName ./internal/job
```

Verified clean on EC2 at commit `a5b0edd`: build, vet, and the full test suite all pass.
`internal/{application,company,config,database,discovery,discovery/greenhouse,discovery/jobspy,job,llm/openrouter,scoring}`
all pass; `cmd/jobclaw` and the three `cmd*` helper packages have no test files.

Live-network tests are opt-in and skip by default:

```bash
JOBCLAW_JOBSPY_LIVE_TEST=1 JOBCLAW_JOBSPY_URL=... go test ./internal/discovery/jobspy
```

## CLI

`cmd/jobclaw/main.go` is the composition root and dispatches these subcommands:

```
discover   jobs list   score     shortlist   approve   reject
application <jobID>    answer {add, list, update}
questionnaire          prepare   submit      resume     job <id>
```

### Submission is preview-by-default

`jobclaw submit <id>` performs a **dry run**: it prints the target employer, the
application state, the resolved questionnaire answers, and the configured
targets, then exits without contacting anyone. Sending requires an explicit flag:

```bash
jobclaw submit 1            # preview only, nothing leaves the machine
jobclaw submit 1 --confirm  # actually submits
```

Exit codes distinguish the outcomes that matter: `0` submitted or previewed,
`1` not submitted with local state unchanged, `2` **ambiguous**. Ambiguous means
the request may have reached the employer. Never resubmit on a `2`; the
application stays locked in `SUBMISSION_IN_PROGRESS` pending manual
reconciliation. Check `errors.Is(err, application.ErrSubmissionAmbiguous)` when
handling this in code.

### Commands that mutate live state

`approve`, `reject`, `application`, `prepare`, `resume`, `submit --confirm`. Back
up `data/jobclaw.db` or point `JOBCLAW_DB_PATH` at a scratch file when testing.

### Commands that need `OPENROUTER_API_KEY`

`resume` and `questionnaire`. `application` no longer requires it: the
application and workspace are created regardless, and the resume is reported as
PENDING and retried later with `jobclaw resume <jobID>`. `prepare` validates only
and makes no network calls.

### Questionnaire ingestion

```bash
jobclaw questionnaire <id> --from-greenhouse   # fetch the live form, no credentials
jobclaw questionnaire <id> --source <path>     # ingest from a local text file
jobclaw questionnaire <id>                     # resolve already-ingested questions
```

`--from-greenhouse` reads Greenhouse's public Job Board API. Only the POST
submission endpoint requires auth, so `JOBCLAW_GREENHOUSE_BASE_URL` needs no
secret and defaults to `https://boards-api.greenhouse.io/v1`.

`JOBCLAW_GREENHOUSE_API_KEY` is issued by the **employer** for their own board. An
applicant cannot obtain one, so automated Greenhouse submission is unavailable and
`MANUAL` is the realistic terminal adapter.

## Environment variables

| Variable | Purpose |
| :--- | :--- |
| `OPENROUTER_API_KEY` | LLM key. The var *name* is itself configurable via `api_key_env` in `config/resume.yaml`. |
| `JOBCLAW_DB_PATH` | SQLite database path (has a default). |
| `JOBCLAW_CANDIDATE_CONFIG` | Override path to `config/candidate.yaml`. |
| `JOBCLAW_PREFERENCES_CONFIG` | Override path to `config/preferences.yaml`. |
| `JOBCLAW_GREENHOUSE_BOARDS` | Comma/newline separated board tokens. Empty tokens are ignored. |
| `JOBCLAW_GREENHOUSE_BASE_URL` | Enables the HTTP Greenhouse form provider when set. |
| `JOBCLAW_GREENHOUSE_API_KEY` | Greenhouse API credential. |
| `JOBCLAW_JOBSPY_URL` | JobSpy MCP server endpoint. |
| `JOBCLAW_JOBSPY_LIVE_TEST` | Set to `1` to run live JobSpy tests. |

Note: `.env.example` is currently empty and does not document any of the above.

## Config files

`config/candidate.yaml`, `config/preferences.yaml`, `config/resume.yaml`. The resume config is where
tailoring boundaries live — which transformations are permitted (rewording, reordering, skill
selection) and which are forbidden (metric changes, invented experience).
