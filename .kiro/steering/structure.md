# Repository Structure & Conventions

## Layering

Every package follows the same progression. Respect it.

```
Domain model  →  Repository / persistence  →  Service / workflow  →  Adapter / external integration
```

## Packages

| Package | Responsibility |
| :--- | :--- |
| `internal/discovery` | Fetch vacancies via pluggable sources, filter through shared matcher, hand to job repo. Subpackages: `jobspy/`, `greenhouse/`. |
| `internal/job` | Core job model, status transitions, SQLite-backed persistence. |
| `internal/company` | Company extraction, name normalization, evidence-based classification. |
| `internal/scoring` | Match candidate context against job requirements and company profile; component + overall scores. |
| `internal/application` | Largest package. Workspace creation, resume tailoring, question ingestion, answer resolution, LLM fallback, readiness gating, transactional submission state. |
| `internal/llm` | LLM prompt execution abstraction. Subpackage: `openrouter/`. |
| `internal/database` | SQLite connections, seed config, migration execution. |
| `internal/config` | Env vars, global app config, resume tailoring boundaries. |
| `cmd/jobclaw` | CLI composition root — wires concrete implementations into services. |
| `config/` | YAML: `candidate.yaml`, `preferences.yaml`, `resume.yaml`. |
| `data/` | Runtime workspaces and SQLite DB. Gitignored. |

Small internal CLIs live in `cmdclassify/`, `cmdinspect/`, `cmdscore/` under their owning packages.

## Conventions

**Concern separation.** Each package defines Go `interface`s, a SQLite implementation file
(`sqlite_repository.go`), and a separate service layer (`service.go`). Follow the existing naming.

**Test placement.** Tests are **colocated with the code** as `*_test.go` in the same package
(e.g. `internal/job/status_transition_test.go`). There is no top-level `tests/` directory.
Tests are primarily table-driven with clear mocks and setup helpers. Mocks live alongside
production code where already established (`mock_resume_generator.go`, `mock_resume_llm.go`).

**Schema migrations.** Never modify schema ad hoc. Add a sequentially numbered file under
`internal/database/migrations/` and let the migrate service in `internal/database/` load it.
Current head: `009_submission_state.sql`.

**Isolate side effects.** Core business logic must never call network APIs or the OS filesystem
directly. Go through interfaces, workspaces under `data/`, and registered adapters such as the
`ApplicationSubmitter` interface and `submission_adapter_registry.go`.

**Platform-specific code stays behind platform boundaries.** Greenhouse is the reference
implementation: `greenhouse_url.go`, `greenhouse_form.go`, `greenhouse_form_provider.go`,
`greenhouse_http_form_provider.go`, `greenhouse_submission_adapter.go`. The core application
workflow remains platform-independent. New ATS adapters plug in at the same seam.

**Context and timeouts.** Operations take `context.Context` with timeouts appropriate to the workflow.

## Reading order for the codebase

1. `cmd/jobclaw/main.go` — how dependencies are wired.
2. `internal/job/` — jobs, repositories, status transitions.
3. `internal/discovery/` — `source.go`, `request.go`, `service.go`, `matcher.go`, then the subpackages.
4. `internal/scoring/`, `internal/company/`.
5. `internal/application/` — read as a pipeline: application → resume → questionnaire → answers →
   preparation → readiness → submission → attempt state → recovery.
6. Greenhouse files as the platform-integration reference.
