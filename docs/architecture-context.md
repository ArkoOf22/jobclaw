# JobClaw Architecture & Implementation Context

> Source document imported into the repo as the archival record behind `.kiro/steering/`.
> The steering files are the distilled, always-loaded version. This is the long form.
> Where this document and the code disagree, see the corrections section in
> `.kiro/steering/implementation-state.md`.

---

## 1. Product Intent & Core Workflows

### 1.1 What JobClaw is

JobClaw is a Go-based backend system designed to bring automation, structural reliability, and
absolute safety guarantees to the job search and application lifecycle. It automates:

1. **Job Discovery:** Retrieving and deduplicating job postings from pluggable sources.
2. **Job Evaluation:** Scoring and matching opportunities against a detailed candidate profile.
3. **Application Workspace Isolation:** Creating a secure, structured workspace for each approved job.
4. **Artifact Tailoring:** Creating highly targeted application assets (such as tailored resumes).
5. **Questionnaire Processing:** Extracting, classifying, and resolving complex application questions
   using candidate facts and LLM fallbacks.
6. **Durable Submission:** Executing atomic state transitions and dispatching applications through
   manual or automated adapters.

### 1.2 End-to-End User Flow

The primary user journey moves through two separate phases separated by an explicit human-in-the-loop
approval gate:

```
[ Discovery Sources ]
         │
         ▼
    DISCOVERED
         │
         ▼
      SCORING ────────► SCORED / SHORTLISTED
                             │
                             ▼ (Candidate Decision)
                      APPROVED or REJECTED
                             │
       ┌─────────────────────┴──────────────────────┐
       │ (Only if APPROVED)                         │ (If REJECTED)
       ▼                                            ▼
     DRAFT (Application Workspace Created)        [Archived/Terminated]
       │
       ├─► Artifact Tailoring (Resume)
       ├─► Questionnaire Ingestion & Resolution
       ▼
 READY_FOR_REVIEW
       │
       ▼ (Readiness Checks Passed)
 READY_TO_APPLY
       │
       ▼ (Atomic State Lock Committed)
SUBMISSION_IN_PROGRESS
       │
       ├───► [External Submission Target]
       │                 │
       ├─── Success ────►│───► APPLIED
       │                 │
       ├─── Def. Failure ├─► Revert to READY_TO_APPLY
       │                 │
       └─── Ambiguous ───┴─► [Manual Reconciliation Locked]
```

---

## 2. Domain & Lifecycle Specifications

### 2.1 The Job Domain

Jobs represent external vacancies retrieved from configured discovery sources. They exist
independently of application workspaces.

Job lifecycles (statuses):

- `DISCOVERED`: Job fetched from an external source but not yet scored.
- `SCORING`: Active evaluation of the job's alignment with candidate profile.
- `SCORED`: Evaluation complete; job is considered a standard fit.
- `SHORTLISTED`: Job matches high-priority criteria.
- `APPROVED`: Candidate has reviewed the job and selected it for active application.
- `REJECTED`: Candidate has archived the job.

Strict state transition rules:

- Transitions are defined in `internal/job/status_transition.go` and enforced in the repository layer.
- A job **cannot** bypass the scoring phase.
- An application **cannot** be created for any job that has not been explicitly moved to `APPROVED`.

### 2.2 The Application Domain

Applications represent the candidate's active, local workspace and preparation workflows. Only
**one application** can exist per job.

Application lifecycles (statuses):

- `DRAFT`: Workspace initialized; tailoring and questionnaire ingestion in progress.
- `READY_FOR_REVIEW`: Under revision or awaiting final validation.
- `READY_TO_APPLY`: Preparation complete, readiness rules met, and ready to submit.
- `SUBMISSION_IN_PROGRESS`: Transactional local lock acquired *before* calling the submission adapter.
- `APPLIED`: Successfully submitted and confirmed by the external system.

---

## 3. Package & Architecture Breakdown

JobClaw strictly separates concerns following the
**Domain Model → Repository / Persistence → Service / Workflow → Adapter / External Integration**
pattern:

- `internal/discovery`: Responsible for fetching vacancies via pluggable sources (such as JobSpy and
  Greenhouse-related clients) and passing raw data to the job repository.
- `internal/job`: Manages the core job model, status transitions, and SQLite-backed job persistence.
- `internal/company`: Extracts company details, standardizes/normalizes company names, and handles
  evidentiary company classification.
- `internal/scoring`: Evaluates the match between candidate context, job requirements, and company
  profiles, generating granular match metrics and overall scores.
- `internal/application`: The largest package. Coordinates workspace directory creation, resume
  tailoring, question ingestion, candidate answer resolution, LLM fallback, readiness gating, and
  transactional state tracking for submissions.
- `internal/llm`: Abstraction layer for LLM prompt execution, integrated with OpenRouter.
- `internal/database`: Coordinates SQLite connections, database seed configuration, and SQL migration
  execution.
- `internal/config`: Orchestrates environment variables, global app configurations, and resume
  tailoring boundaries.

---

## 4. Key Design Decisions & Rationales

### 4.1 SQLite for Durable Persistence

**Rationale:** Rather than relying on highly volatile, in-memory state machines or long-running agent
threads (which can lose state during network blips or crash loops), JobClaw stores all transient,
operational, and historical states in SQLite.

**Benefits:** Applications can survive process restarts, crash-loops, and terminal errors. A restart
simply reloads the active application from its persisted state (e.g., restoring `READY_TO_APPLY` or
recognizing an active `SUBMISSION_IN_PROGRESS`).

### 4.2 Abstracted LLM Layer & OpenRouter

**Rationale:** Avoids tight coupling to single model providers (like OpenAI or Anthropic).

**Benefits:** Features like resume tailoring and questionnaire fallback are coded against interfaces
in `internal/llm`. The backend utilizes OpenRouter, enabling model swapping via environment
configurations without rewriting workflow logic.

### 4.3 Multi-Component Scoring Model

**Rationale:** Matches candidates against multiple vectors to produce trustworthy, explainable scores.

**Benefits:** Instead of generating a single arbitrary number, the scoring package calculates
independent component grades for:

- Required skills matching
- Years of experience and seniority alignment
- Compensation ranges
- Geographical/location preferences
- Granular AI recommendations and reasoning

### 4.4 Human-In-The-Loop (HITL) Gateways

**Rationale:** Automated job hunting can easily run out of control, submitting poor matches or
violating platform policies.

**Benefits:** By introducing a hard boundary between `job` and `application` via the `APPROVED`
status, the user maintains absolute control. No workspace assets are generated and no submissions are
executed without explicit human consent.

---

## 5. Safety Principles, Constraints, & Non-Goals

### 5.1 The Submission Boundary & Ambiguity

External platforms do not support participating in local database transactions. If a process crash,
timeout, or network failure occurs during submission, the status is left uncertain. JobClaw handles
this boundary using a multi-step transaction protocol:

1. **Durable Intent Lock:** Before crossing the external network, a local SQLite transaction changes
   the status to `SUBMISSION_IN_PROGRESS` and registers a `submission_attempt_id` and
   `submission_attempted_at`.
2. **External Crossing:** The submission adapter is called.
3. **Strict Outcome Separation:**
   - *Confirmed Success:* State updates to `APPLIED`.
   - *Confirmed Failure:* State reverts to `READY_TO_APPLY`.
   - *Ambiguity (Timeout/No Response):* The state **remains locked**. The system refuses to execute
     automatic retries to avoid duplicate submissions, ToS violations, or spam flags. The attempt must
     be resolved through manual reconciliation.

### 5.2 Constraints & Non-Goals

- **No Auto-Apply Spammers:** JobClaw rejects automated bulk-applying. It is built as a precision
  assistant for highly targeted applications.
- **No Arbitrary Retries:** Redoing submissions blindly is a non-goal. Every network boundary
  interaction is tracked durably.

---

## 6. Codebase Conventions

- **Concern Separation:** Every package has a clear pattern of defined Go `interface`s, an SQLite
  implementation file, and a separate service layer.
- **Table-Driven Tests:** Tests are primarily table-driven, isolating assertions with clear mocks and
  setup helpers. (Note: they live colocated in-package, not under a top-level `tests/` directory.)
- **Schema Migrations:** Database schema modifications must never be done ad-hoc. All modifications
  must be declared sequentially inside `internal/database/migrations/` (currently up to
  `009_submission_state.sql`) and loaded via the migrate service in `internal/database/`.
- **Isolate Side-Effects:** Never let core business logic directly call network APIs or OS
  filesystems. Use interfaces, workspaces under `data/`, and registered adapters (like the
  `ApplicationSubmitter` interface).

---

## 7. Original Implementation Audit

Retained as the imported baseline. Three rows were inaccurate against the tree; see
`.kiro/steering/implementation-state.md` for the reconciled version.

| Feature | Status | Code File (Source of Truth) | Missing / Next Steps |
| :--- | :--- | :--- | :--- |
| **Job Statuses** | VERIFIED | `internal/job/status.go`, `internal/job/job.go` | None. |
| **Job Transitions** | VERIFIED | `internal/job/status_transition.go` | None. |
| **App Statuses** | VERIFIED | `internal/application/application.go` | None. |
| **App Transitions** | VERIFIED | `internal/application/service.go` | None. |
| **Approval Gating** | VERIFIED | `internal/application/service.go` | Enforces `APPROVED` state check. |
| **Duplicate Checking** | VERIFIED | `internal/application/service.go` | Enforces single application per job. |
| **Resume Tailoring** | VERIFIED | `internal/application/resume_generator.go` | Separate coverage for the pure generator is minimal. |
| **Fact Guarding** | VERIFIED | `internal/application/resume_fact_guard.go` | None. |
| **Cover Letters** | DOCUMENTATION ONLY | None. `artifact.go` reserves the directory; no generator exists. | Needs service implementation. |
| **JobSpy Discovery** | PARTIAL | `internal/discovery/jobspy/client.go` | Lacks unit/mock tests. MCP wiring incomplete. |
| **Greenhouse Disc.** | PARTIAL | `internal/discovery/greenhouse/client.go` | Client written but untested. |
| **Job Scoring** | VERIFIED | `internal/scoring/scorer.go`, `service.go` | None. |
| **Company Class.** | VERIFIED | `internal/company/service.go`, `classifier.go` | Normalizer unit tests are light. |
| **Questionnaires** | VERIFIED | `internal/application/questionnaire_service.go` | None. |
| **Answer Bank** | VERIFIED | `internal/application/answer_repository.go` | None. |
| **LLM Answer Fallback** | VERIFIED | `internal/application/answer_resolver.go` | None. |
| **Submission Registry** | VERIFIED | `internal/application/submission_adapter_registry.go` | None. |
| **Manual Submitter** | VERIFIED | `internal/application/manual_submission_adapter.go` | None. |
| **Greenhouse Submitter** | PARTIAL *(claimed untracked — actually committed in `33821f5`)* | `internal/application/greenhouse_submission_adapter.go` | Registry wiring. |
| **SQLite Persistence** | VERIFIED | `internal/database/database.go` | Migrations `001`–`009` defined. |
| **Atomic Transactions** | VERIFIED | `internal/application/sqlite_submission_transaction.go` | None. |
| **CLI Commands** | PARTIAL *(claimed skeletal — actually 14+ subcommands wired)* | `cmd/jobclaw/main.go` | Lacks test coverage. |
