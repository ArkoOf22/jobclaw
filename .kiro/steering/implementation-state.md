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
