# JobClaw — Product Intent

JobClaw is a Go backend that automates the job search and application lifecycle while keeping
consequential decisions under human control. It is a precision assistant, not a mass-application bot.

North star:

> Automate repetitive work, preserve human control, never invent candidate information, and make
> consequential state transitions observable and recoverable.

## Six responsibilities

1. **Job discovery** — fetch and deduplicate postings from pluggable sources.
2. **Job evaluation** — score opportunities against a detailed candidate profile.
3. **Workspace isolation** — create a structured workspace per approved job under `data/`.
4. **Artifact tailoring** — generate targeted assets (tailored resumes) without inventing facts.
5. **Questionnaire processing** — extract, classify, and resolve application questions from verified
   candidate facts, with LLM fallback.
6. **Durable submission** — atomic state transitions, dispatch through manual or automated adapters.

## End-to-end flow

The journey splits into two phases separated by an explicit human approval gate.

```
Discovery sources
      ↓
  DISCOVERED → SCORING → SCORED / SHORTLISTED
                              ↓  (candidate decision)
                     APPROVED  or  REJECTED → archived
                              ↓
                            DRAFT   (workspace created)
                              ├─ resume tailoring
                              ├─ questionnaire ingestion + resolution
                              ↓
                      READY_FOR_REVIEW
                              ↓  (readiness checks pass)
                       READY_TO_APPLY
                              ↓  (atomic state lock committed)
                  SUBMISSION_IN_PROGRESS
                              ↓
                     ┌────────┼────────────────┐
                  success  definite failure  ambiguous
                     ↓        ↓                ↓
                  APPLIED  READY_TO_APPLY   stays locked,
                                            manual reconciliation
```

## Domain boundaries

**Jobs** are external vacancies. They exist independently of application workspaces.
Statuses: `DISCOVERED`, `SCORING`, `SCORED`, `SHORTLISTED`, `APPROVED`, `REJECTED`.

- Transitions are declared in `internal/job/status_transition.go` and enforced in the repository layer.
- A job cannot bypass scoring.
- An application cannot be created for a job that is not explicitly `APPROVED`.

**Applications** are the local preparation workspace. Exactly one application per job.
Statuses: `DRAFT`, `READY_FOR_REVIEW`, `READY_TO_APPLY`, `SUBMISSION_IN_PROGRESS`, `APPLIED`.

## Safety principles

These are load-bearing. Do not relax them for convenience.

- **Human-in-the-loop gate.** The `APPROVED` boundary between `job` and `application` is a hard wall.
  No workspace assets are generated and no submission executes without explicit human consent.
- **Candidate truth.** Tailoring may reword, reorder, and select. It may not invent experience,
  metrics, or history. See `internal/application/resume_fact_guard.go`.
- **Preparation ≠ execution.** Prepared data must be inspectable and validatable before side effects.
- **Ambiguity is a real state.** When an external submission outcome is unknown, the state stays
  locked. Never auto-retry: that risks duplicate submissions, ToS violations, and spam flags.
  Resolution is manual.
- **Source isolation.** One failing discovery provider must not stop the others.

## Non-goals

- Automated bulk applying.
- Blind or arbitrary submission retries.
- Any flow that silently converts an unknown outcome into success or failure.

## Current development priority

Prove one complete end-to-end vertical slice before building more isolated features:

```
Discover → Store → Score → Approve → Create application → Resume → Form
→ Questions → Answers → Prepare → Readiness → Submit → Outcome
```

The work is finding missing *orchestration* between components that already exist. Prefer wiring
over new features.
