# Tech Stack & Commands

## Stack

- **Go** — `go.mod` declares `go 1.26.5`, module name `jobclaw`.
- **SQLite** via `modernc.org/sqlite` (pure Go, no cgo).
- **LLM** via OpenRouter, behind interfaces in `internal/llm`. Model is swappable through config,
  never hardcoded into workflow logic.
- **MCP** via `github.com/modelcontextprotocol/go-sdk` — used for JobSpy discovery.
- **Config** via `gopkg.in/yaml.v3`.

Direct dependencies are few and deliberate. Prefer the standard library before adding anything new.

## Toolchain requirement (read this first)

`go.mod` requires Go >= 1.26.5. The currently installed toolchain is **1.25.5** with
`GOTOOLCHAIN=local`, so `go build`, `go vet`, and `go test` all fail immediately with:

```
go: go.mod requires go >= 1.26.5 (running go 1.25.5; GOTOOLCHAIN=local)
```

This is an environment gap, not a code problem. Resolve it before trusting any build or test result:
install Go 1.26.5+, or unset `GOTOOLCHAIN=local` so Go can fetch the required toolchain automatically.
Do not report tests as passing without an actual clean run.

## Commands

```bash
go build ./...
go vet ./...
go test ./...                    # full suite
go test ./internal/application   # single package
go test -run TestName ./internal/job
```

`go test ./...` and `go vet ./...` were clean as of commit `a5b0edd`.

Live-network tests are opt-in and skip by default:

```bash
JOBCLAW_JOBSPY_LIVE_TEST=1 JOBCLAW_JOBSPY_URL=... go test ./internal/discovery/jobspy
```

## CLI

`cmd/jobclaw/main.go` is the composition root and dispatches these subcommands:

```
discover   jobs      score     shortlist   approve   reject
application          answer {add, list, update}
questionnaire        prepare   submit      resume     job
```

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
