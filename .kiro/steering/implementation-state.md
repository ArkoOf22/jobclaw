---
inclusion: always
---

# Implementation State

Baseline audit, reconciled against the working tree at commit `a5b0edd` (branch `main`, in sync with
`origin/main`, working tree clean apart from untracked `.superbrain/`).

This file goes stale. When it conflicts with the code, the code wins — and update this file.

## Verified

Implementation and test coverage both confirmed present.

| Area | Source of truth |
| :--- | :--- |
| Job statuses | `internal/job/status.go`, `internal/job/job.go` |
| Job transitions | `internal/job/status_transition.go` |
| Application statuses | `internal/application/application.go` |
| Application transitions | `internal/application/service.go` |
| Approval gating | `internal/application/service.go` |
| Duplicate application check | `internal/application/service.go` |
| Resume tailoring | `internal/application/resume_generator.go`, `llm_resume_generator.go` |
| Resume fact guarding | `internal/application/resume_fact_guard.go` |
| Job scoring | `internal/scoring/scorer.go`, `service.go`, `rules.go` |
| Company classification | `internal/company/service.go`, `classifier.go` |
| Questionnaire pipeline | `internal/application/questionnaire_service.go`, `questionnaire_extractor.go`, `questionnaire_ingestor.go` |
| Answer bank | `internal/application/answer_repository.go` |
| LLM answer fallback | `internal/application/answer_resolver.go` |
| Submission adapter registry | `internal/application/submission_adapter_registry.go` |
| Manual submitter | `internal/application/manual_submission_adapter.go` |
| Greenhouse submission adapter | `internal/application/greenhouse_submission_adapter.go` |
| SQLite persistence | `internal/database/database.go`, migrations `001`–`009` |
| Atomic submission transaction | `internal/application/sqlite_submission_transaction.go` |
| Shared discovery matcher | `internal/discovery/matcher.go` |
| Concurrent discovery + source isolation | `internal/discovery/service.go` |

Thin spots inside otherwise-verified areas: standalone unit tests for the pure resume generator and
for `internal/company/normalizer.go` are light.

## Partial

- **JobSpy discovery** — `internal/discovery/jobspy/client.go` exists; only a live test that skips by
  default. No mock/unit coverage. MCP wiring incomplete.
- **Greenhouse discovery client** — `internal/discovery/greenhouse/client.go` written. Board-level
  behavior is covered, the raw client is not directly unit-tested.

## Missing

- **Cover letters** — the slot is reserved (`CoverLetterPath` on the application model,
  `ArtifactCoverLetter` in `artifact.go`, `cover_letter_path` column from migration `005`) but no
  generator service exists. Purely a schema placeholder today.

## Corrections to the original bootstrap doc

The imported context document contains three claims that do not match the tree. Trust this section:

1. **Test locations.** The doc cites a top-level `tests/` directory (`tests/job/job_test.go`,
   `tests/application/service_test.go`). No such directory exists. All tests are colocated in-package.
2. **Greenhouse submission adapter is not untracked.** The doc flags it as working-tree-only code
   needing commits. It is committed (`33821f5`) and `git ls-files` confirms both the adapter and its
   test are tracked. Registry wiring is the only open question.
3. **CLI is further along than "placeholder/skeletal."** `cmd/jobclaw/main.go` dispatches
   `discover`, `jobs`, `score`, `shortlist`, `approve`, `reject`, `application`, `answer`
   (`add`/`list`/`update`), `questionnaire`, `prepare`, `submit`, `resume`, and `job`. What it lacks
   is test coverage, not commands.

## Live observations (verified on EC2, 2026-08-26)

Build, vet, and the full test suite pass on the EC2 host at `a5b0edd`.

**The production DB is polluted with pre-matcher results.** `internal/discovery/matcher.go` was added
in `a5b0edd` (today). The 20 jobs in `data/jobclaw.db` were discovered around Aug 21 by the older
Greenhouse client from `66b181d`, which had no keyword filtering. They are all off-target for a
backend candidate: "University Recruiter", "Workplace Operations Manager", "U.S. Federal Government
Relations Director", "Technical Program Manager", and similar.

Consequence: **job 7 is "Workplace Operations Manager - Dublin"**, and it was approved with a tailored
resume generated against it at `data/applications/7/resume/tailored_resume.txt`. The pipeline worked
mechanically on meaningless input. Job 7 is not a valid subject for the vertical slice.

**The new matcher is correct.** A scratch-DB discovery run (`JOBCLAW_DB_PATH=/tmp/...`,
`JOBCLAW_GREENHOUSE_BOARDS=stripe`) returned 3 jobs instead of 20, all genuine:
Backend Engineer (Core Technology), Backend Engineer (Financial Connections), Backend Engineer
(Payments). Keyword filtering against `target_roles.primary` works as intended.

**Source isolation is confirmed working in the wild.** In the same run, the JobSpy source failed with
`dial tcp 127.0.0.1:8000: connect: connection refused` while Greenhouse succeeded and stored its
results. Exactly the designed behavior.

**JobSpy is dead until its server runs.** The MCP server exists at `/home/openclaw/jobspy-mcp` with a
`.venv` but is not running. Discovery is Greenhouse-only in practice.

**Discovery is capped at 10 per source** (`Limit: 10`, hardcoded in `runDiscover`) with
`HoursOld: 168` and `RemoteOnly: false`. Not configurable from YAML yet.

## Vertical-slice trace (EC2, scratch DB, 2026-08-26)

Traced `discover -> score -> approve -> application -> prepare -> submit` against a
throwaway DB. The live database was backed up first and never touched.

Four defects found and fixed:

1. **Company classification was never invoked.** `ScoreJob` reads
   `Company.Classification`, and with `require_product_company: true` a non-PRODUCT
   value forces SKIP regardless of score. Nothing in the CLI ever called
   `company.Service.Classify`, so every company stayed UNKNOWN and **every job was
   skipped, permanently**. All the pieces existed and were tested; nothing wired
   them together. Scoring now classifies on demand via `WithCompanyClassifier`.
   Stripe correctly resolves to PRODUCT (`company` component 2.5 -> 5.0).
2. **APPLY left jobs in a weaker status than SHORTLIST.** Only SHORTLIST advanced
   the job; APPLY fell through to SCORED. Extracted to `targetStatusFor`.
3. **Resume generation was fused into application creation.** `CreateForApprovedJob`
   committed the application row and then generated the resume. On LLM failure the
   caller got an error while the row persisted, and the retry matched the "already
   exists" branch and returned success without generating anything, leaving the
   application permanently resume-less while reporting success. Split into
   `CreateForApprovedJob` + idempotent `EnsureTailoredResume`, which also
   regenerates when the recorded artifact is missing or empty.
4. **Ambiguous submissions were reported as "No application was submitted."**
   The domain layer handles ambiguity correctly, but the CLI collapsed every error
   into that message, inviting a duplicate application. Added
   `ErrSubmissionAmbiguous` for `errors.Is`, and the lock check now runs *before*
   the readiness check, since a locked application otherwise fails as
   "not READY_TO_APPLY" and hides the only fact that matters.

Also confirmed working: readiness correctly blocks on a missing resume, source
isolation survives a dead JobSpy, and the `submit` safety chain refuses an unready
application even with `--confirm`.

### Open: scoring is calibrated out of reach

Thresholds are `shortlist: 70`, `apply: 80`, over a 100-point scale. Genuine
Backend Engineer roles at Stripe score **68.5, 68.0, and 51.0** after the
classification fix, so nothing reaches even SHORTLIST.

Two components are structurally unreachable for Greenhouse-sourced jobs:

- `compensation` returns a flat 5/10 when no salary is present, and Greenhouse
  never supplies salary. Absence of evidence is scored as mediocre evidence.
- `skills` (max 20) and `domain` (max 10) need many distinct keyword hits;
  real descriptions land around 7 and 3.

This is a calibration decision, not a defect, so it is left alone. Options:
lower the thresholds, or renormalize over the signals actually present rather
than a fixed 100. Note also that the comment above `maxRawScore` claims the
components total 110 when they total 100, which suggests the scale changed at
some point without the thresholds being revisited.

Not a blocker for the slice: approval is a manual human gate and
`SCORED -> APPROVED` is a legal transition, so a SKIP recommendation is advice
rather than a wall.

### Still blocked

`OPENROUTER_API_KEY` is absent, so resume tailoring and LLM answer fallback
cannot run. Everything up to and including readiness validation now works
without it. Set it with `scripts/set-openrouter-key.sh`.

Questionnaire ingestion was not exercised: it needs `JOBCLAW_GREENHOUSE_BASE_URL`
plus an API key. Readiness currently passes questionnaire checks with "no
questionnaire questions", which is a soft failure mode worth tightening once
ingestion runs, since nothing distinguishes "no questions" from "not yet ingested".

## Roadmap position

Phases 1–3 (foundation, discovery, application preparation) are largely complete. Phase 4, the
Greenhouse vertical slice, has all core pieces implemented.

The gating task is Phase 4's last step: prove the full real-world flow end to end and find the
missing orchestration between existing components. Do not start Phase 5 (scheduling, approval
queues, reporting) or Phase 6 (additional ATS adapters: Lever, Ashby, Workday, SmartRecruiters)
before that slice runs.

Deferred by design: answer resolution engine refinement (known → verified answer, similar → safe map,
unknown → require review, ambiguous → never guess), resume artifact hardening, richer company
intelligence signals, and the operational dashboard.
