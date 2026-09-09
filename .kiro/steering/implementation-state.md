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

Also confirmed working: source isolation survives a dead JobSpy, and the `submit`
safety chain refuses an unready application even with `--confirm`.

### Readiness hardening

Readiness was declaring applications ready on evidence it did not have. Three
holes, all closed:

1. **Zero questions counted as complete.** The check reported
   `✓ questionnaire — no questionnaire questions`, which cannot distinguish a form
   with no questions from a form that was never fetched. A Greenhouse application
   could pass readiness and be submitted entirely blank. Readiness now consults
   the event log for `QUESTIONNAIRE_INGESTED` and blocks when ingestion cannot be
   confirmed, via `WithEventRepository`.
2. **Status was trusted over content.** A question marked `ANSWERED` or `APPROVED`
   with an empty answer passed. Blank and whitespace-only answers now block
   regardless of status.
3. **A recorded artifact path was treated as proof of existence.** The check only
   verified the path string was non-empty, and its reason read "artifact path is
   configured". A deleted or truncated resume passed readiness. Now `os.Stat`
   confirms the file exists and is non-empty.

Added advisory only, deliberately not a blocker: an `answer_provenance` check
surfaces LLM-generated answers that are not candidate-verified. Blocking would
stall the workflow, so the decision sits with the operator reviewing the dry-run
payload. Consider requiring `QuestionApproved` for LLM answers before real
submissions.

Four existing readiness tests and one preparation test were asserting the old
permissive behavior, which is why these holes survived. They used placeholder
paths like `"resume.txt"` that never existed on disk. Updated to write real
temp-file artifacts.

Separately, tests were writing `data/applications/...` into the source tree
because workspaces are created relative to the working directory. Leftovers there
can mask or unmask failures between runs. Added a `useTempWorkingDir` helper and
applied it to every test that creates a workspace.

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

### Settled: automated Greenhouse submission is not achievable

Investigated empirically, read-only, on 2026-08-26. Do not re-litigate this; the
constraint is external and not a gap in JobClaw.

The public application form is a React app at
`job-boards.greenhouse.io/embed/job_app?for={board}&token={job_id}`. Its `<form>`
carries `method="get"` as a placeholder; the real submission is a JavaScript call.
Reading the client bundle, that call requires all of:

```
submitPath, csrfToken, fingerprint, recaptchaClient,
securityCode, captchaFailed, jobApplicationRequestToken
```

and attaches `g-recaptcha-enterprise-token` from `recaptchaClient.performAssessment()`.

That closes both candidate paths:

- **Replicating the POST server-side** is impossible. The payload needs a
  **reCAPTCHA Enterprise** token, which is minted by Google against a live browser
  session. It cannot be produced from a server.
- **Driving a headless browser** does not work either. This is invisible,
  score-based reCAPTCHA rather than a solvable checkbox: it scores browser
  fingerprint, IP reputation, and behaviour. Headless Chromium on an EC2
  datacenter IP is close to the worst-scoring combination. A separate
  `fingerprint` field is collected independently of reCAPTCHA, and the code has
  explicit `captchaFailed` and `captcha_retried` handling, so Greenhouse both
  expects and polices automation attempts.

Defeating this would mean circumventing an access control the employer
deliberately enabled. Out of scope regardless of feasibility.

**Consequence for the product.** The realistic terminal step is a one-tap human
submit: JobClaw prepares and validates everything, then hands over the apply URL,
the resume file, and every resolved answer ready to paste. `MANUAL` is the correct
adapter, not a placeholder for missing work.

### Settled: no major ATS permits programmatic submission

Checked Lever, Ashby, and Workday read-only on 2026-08-26 to see whether any of
them allow what Greenhouse does not. None do. This is an industry norm, not a
Greenhouse quirk, so do not re-investigate per platform.

| Platform | Read API | Submission barrier |
| :--- | :--- | :--- |
| Greenhouse | Public, no auth | reCAPTCHA Enterprise (invisible, score-based), plus a separate fingerprint field and a per-request token |
| Lever | Public, no auth | hCaptcha, actively rendered; the submit button is gated on a populated `hcaptchaResponseInput` |
| Ashby | Public, no auth | reCAPTCHA with configured site keys, plus explicit `RejectBase64EncodedResumes` front-end flags |
| Workday | SPA, no clean public API | Requires creating an account per employer before applying |

Lever is a real `multipart/form-data` POST form rather than a pure JS call, which
makes it look more tractable than Greenhouse. It is not: hCaptcha is required, and
the page comments explicitly note that calling `submit()` directly would bypass
validation, so the token cannot be sidestepped.

Ashby's `RejectBase64EncodedResumesFrontEnd` flag is worth noting: it is an
explicit measure against programmatically attached resumes, i.e. exactly this use
case.

**What this means.** Automated application submission is not achievable on any
platform JobClaw would target. The product's terminal step is a one-tap human
submit, permanently. `MANUAL` is the correct adapter everywhere.

**What is still worth building.** Every platform above has a clean, public,
unauthenticated read API. Phase 6 has real value for *discovery and form reading*:

- Lever: `https://api.lever.co/v0/postings/{company}?mode=json` — verified, 384
  postings from `leverdemo` with no credentials, and each posting carries an
  `applyUrl`.
- Ashby: `https://api.ashbyhq.com/posting-api/job-board/{company}` — verified
  against `linear` and `notion`.

Both are straightforward additions to `internal/discovery` following the existing
`Source` interface, and both would extend questionnaire ingestion the same way the
Greenhouse form provider does.

### Greenhouse: the API path also requires an employer key

Per the [Job Board API docs](https://docs.greenhouse.io/job-board.html), only the
POST submission endpoint requires auth. **Every GET is public**, including a job's
application questions. Verified against Stripe's live board with no credentials:
`GET /v1/boards/stripe/jobs/6042172?questions=true` returns 200 with 16 questions.

So `JOBCLAW_GREENHOUSE_BASE_URL` needs no secret and now defaults to
`https://boards-api.greenhouse.io/v1`.

`JOBCLAW_GREENHOUSE_API_KEY` is a **Job Board API Key issued by the employer** from
their own Greenhouse settings. An applicant cannot obtain one for a company they do
not work for. Automated Greenhouse submission is therefore unavailable by design,
and `MANUAL` is the realistic terminal adapter. The remaining honest options are a
prepared payload plus manual submission, or browser automation against the public
form, which is fragile and ToS-sensitive.

`runSubmit` now registers the Greenhouse adapter only when an API key is present.
Gating on the base URL used to abort the whole command, including the dry run,
whenever a URL was set without a key.

### The form-to-questionnaire bridge was missing

The HTTP form provider and the questionnaire ingestor both existed and were
tested, but nothing connected them. The provider was wired solely into the
submission adapter, the one path needing the impossible API key, so questions
could only be ingested from a hand-written local text file.

Added `GreenhouseFormToQuestionnaireInputs` plus
`jobclaw questionnaire <id> --from-greenhouse`. Artifact-backed fields (`resume`,
`resume_text`, `cover_letter`, `cover_letter_text`) and `input_file` types are
skipped, since answer resolution can never satisfy them and they would block
readiness forever. Field metadata retains the original name including any `[]`
multi-select suffix, plus type, required flag, and option labels.

Board tokens cannot always be derived from the job URL: employers often front
their board on their own domain, as Stripe does with
`stripe.com/jobs/search?gh_jid=...`, where the token is absent entirely. Added
`SetBoardToken` and fall back to `JOBCLAW_GREENHOUSE_BOARDS` when exactly one
board is configured. **Proper fix, not yet done:** persist the board token on the
job at discovery time, where it is already known, rather than re-deriving it from
a display URL. That needs a migration.

### Preparation was erasing answers

`Prepare` unconditionally re-ran answer resolution, and the resolver only knows
the verified answer bank. Any answer from another source resolved to NEEDS_REVIEW
and was overwritten with an empty string, so **every `prepare` wiped the answers
the preceding `questionnaire` run had produced.** Observed live: 7 resolved
answers reduced to 0.

`ProcessApplication` now skips questions that already carry a non-empty answer,
the same way it already skipped approved ones. That makes it idempotent, stops
repeat LLM calls on every invocation, and preserves work. Unverified answers are
still surfaced by the `answer_provenance` readiness check and in the submission
dry run, so they get human review before anything is sent.

### Verified end-to-end run

Full slice against a scratch DB with real credentials, live Greenhouse data, and
real LLM calls:

```
discover       3 Backend Engineer roles, JobSpy failed in isolation
score          68.5 / 68.0 / 51.0, classification wired, all SKIP on thresholds
approve        manual gate, SCORED -> APPROVED
application    workspace created, 71-line tailored resume generated
questionnaire  14 questions from Stripe's live form, 7 answered, 7 NEEDS_REVIEW
prepare        BLOCKED, answers preserved, provenance surfaced
submit         dry run prints full payload, nothing sent
```

Resume generation succeeded with `data_collection: deny` and `zdr: true`, so a
compliant provider was available for the pinned model.

The 7 unresolved questions are Email, Phone, work authorization, visa sponsorship,
remote intent, WhatsApp opt-in, and US city/state. All require real personal facts,
and the system correctly refuses to invent them. Supply them with
`jobclaw answer add <field_key> <answer>` to reach READY_TO_APPLY.

### Still blocked

`OPENROUTER_API_KEY` is absent, so resume tailoring and LLM answer fallback
cannot run. Everything up to and including readiness validation now works
without it. Set it with `scripts/set-openrouter-key.sh`.

Questionnaire ingestion has still not been exercised against a real form: it needs
`JOBCLAW_GREENHOUSE_BASE_URL` plus `JOBCLAW_GREENHOUSE_API_KEY`. Readiness now
blocks until ingestion is confirmed, so this is the next hard requirement for a
complete slice rather than something that can be skipped.

Current end-to-end position on a scratch DB, with no credentials set:

```
discover  OK   3 genuine Backend Engineer roles via Greenhouse
score     OK   classification wired, 68.5 / 68.0 / 51.0, all SKIP on thresholds
approve   OK   manual gate, SCORED -> APPROVED
application OK application + workspace created, resume PENDING
prepare   OK   correctly BLOCKED on: resume missing, questionnaire not ingested
submit    OK   dry run prints payload; --confirm refuses an unready application
```

Everything mechanical works. What remains is credentials and scoring calibration.

## Scheduled-run defects found and fixed (2026-09-06)

The system looked healthy — the timer ran every six hours, exited 0, and the
journal filled with scored jobs — while two of its three stages were broken. Both
failures were silent in the sense that mattered: `systemctl status` showed
`SUCCESS` either way.

### Greenhouse discovery had produced nothing for nine days

`JOBCLAW_GREENHOUSE_BOARDS` contained `phonepe`, which is not a Greenhouse board
at all: both `/v1/boards/phonepe` and `/v1/boards/phonepe/jobs` return 404.
`MultiBoardClient.Discover` returned `nil, err` on the first board that failed, so
one dead token discarded the jobs already collected from the eleven healthy boards
and reported `Fetched: 0`. The newest Greenhouse row in the database was dated
Aug 28.

The consequence was a quality collapse rather than an outage. JobSpy kept working,
so the queue still filled — with Wipro `DEVELOPER L3`, mainframe and ASP.NET roles
— while the curated employer boards that justify the whole Greenhouse path
contributed nothing.

Three changes:

- `MultiBoardClient.Discover` now attempts every board, returns partial results
  alongside `errors.Join` of the per-board failures, and names the board in each
  error. `greenhouse board returned HTTP 404` is not actionable across a dozen
  boards. It still stops early if `ctx` is done, so a cancelled run does not
  report twelve identical deadline errors.
- `discovery.Service` used to `return` before `Upsert` whenever `Discover`
  errored, which threw away partial results. It now stores what arrived and
  *then* reports the error. A source may legitimately return jobs and an error
  together.
- `Result.Partial()` plus a `PARTIAL` status in `runDiscover`, because
  `Status: FAILED` printed next to `Stored: 114` reads as a contradiction.

Note the isolation the product promises is per-`Source`, and all boards sit behind
one `Source`. Service-level isolation could never have caught this; it had to be
fixed inside the composite. Regression tests are in
`internal/discovery/greenhouse/multi_board_test.go` and put the failing board
first, so a return to fail-fast is caught.

Verified: `Fetched: 114, Stored: 114, Status: OK`, with fresh rows from GitLab,
Zscaler, Twilio, Stripe and Airbnb.

### Every scheduled sheet sync failed on the gog keyring

`jobclaw-discover.service` sets `ProtectHome=read-only` and listed only
`ReadWritePaths=/home/openclaw/jobclaw`. `gog` keeps its OAuth refresh token in a
file-backed keyring at `/home/openclaw/.local/share/gogcli/keyring` and opens a
`.lock` file to read it, so every sync from the timer died with
`open ... /keyring/.lock: read-only file system`.

What made this expensive to find: **a manual run works**, because an interactive
shell has a writable home. The failure existed only under systemd. The wrapper
also treats sheet sync as best-effort, so the run still exited 0. The only
evidence was in `journalctl`, and only on runs that actually had rows to append —
a run with zero new rows never called `gog` and looked clean.

Fixed by adding the three `gogcli` directories to `ReadWritePaths`. Note gog needs
*write* access there, not just read: it rewrites the stored token on every access
token refresh. Verified with a real timer-triggered run: `New rows: 2`,
`Appended 2 row(s).`, no keyring error, no duplicate rows in the sheet.

### The timer never ran on the schedule it documented

The unit carried `OnCalendarTimezone=Asia/Kolkata`, which is not a systemd
directive. It was silently ignored and the timer ran on UTC, firing at
05:30/11:30/17:30/23:30 IST instead of the documented 00/06/12/18. Per-timer
timezones need systemd 252; the host runs 249. Now expressed as
`OnCalendar=*-*-* 00,06,12,18:30:00` in UTC, which lands on the intended IST
times. `systemd-analyze verify` flags unknown keys and is worth running after any
unit edit.

### Checked and deliberately left alone

- 60 jobs sit at status `SHORTLISTED` with a latest score of `SKIP`, so they never
  reach the sheet: `ListUnsyncedForSheet` filters on the score, while status only
  ever advances. Stale state from a threshold change, not a live defect. As of the
  experience-filter fix below this is 66.

## The experience filter never worked (2026-09-09)

Postings demanding four, five, and eight years were reaching the shortlist. Three
independent defects, each sufficient on its own, and the middle one hid the others.

**Scoring read escaped HTML.** Descriptions are stored exactly as the source
returned them, and Greenhouse returns HTML that is itself escaped. `internal/scoring`
never normalised it, so every word-oriented rule saw markup welded to the word
beside it: `&lt;li&gt;8+` instead of `8+`, and `&lt;strong&gt;Go&lt;/strong&gt;`
which `containsTerm` cannot equal to `go`. The old `extractMinimumYears` required
the whitespace-delimited token before `years` to parse as a number, so it found
nothing on any Greenhouse posting. Stripe job 701 asks for "8+ years of
experience", scored `experience=15.0` — full marks — and shortlisted at 77.3.

`NormalizeJobDescription` already existed but lived in `internal/application`,
reachable only from the LLM prompt path. Moved to `job.NormalizeDescription`;
the application-package function now delegates. `Scorer.Score` normalises once and
every rule reads the same plain text.

**The veto allowed a two-year stretch.** `experienceStretchYears = 2.0` was
hardcoded, so with a two-year candidate the veto fired only above four years and
every "3-4 years" and "4-5 years" posting passed by design. Now
`job_preferences.experience.max_required_years` in `config/preferences.yaml`, set
to 2. Unset falls back to the candidate's `total_years`, so an absent key tightens
the filter rather than disabling it.

**Extraction was too narrow and stopped too early.** Only the literal word
"year" was matched, so `_4+Yrs_` in a Naukri title read as no requirement; and only
the first mention anywhere was read. `internal/scoring/experience.go` replaces it:
regex over `years|yrs|year(s)`, ranges on `-`, en/em dash and `to`, underscores
flattened first because Go's `\b` does not fire between `s` and `_`.

Semantics worth not re-litigating:

- **A range counts by its low end.** "2-4 years" is genuinely open to a two-year
  candidate and survives. This is why 12 visible jobs record `required_years=2.0`.
- **Across several mentions the highest wins.** A posting listing "3-5 years
  backend" and "8+ years distributed systems" requires both.
- **A figure in the description needs requirement language within ~60 bytes before
  or ~80 after** (`experienceContextTerms`). Amazon job 2135's "Our 3 year vision
  is to be best-in-class" is prose, and vetoing on it would hide a real job. Years
  in a *title* bypass the check — a title naming years is naming the requirement.
- **Schooling is excluded on a much tighter 25-byte window.** "15 years full time
  education" is standard in Indian JDs. The first attempt shared the 80-byte
  window, and "Educational Qualification" on the following line then suppressed the
  genuine "Minimum 5 Year(s) Of Experience" above it, keeping ten Accenture
  postings visible. Schooling language always sits against its own figure.

`required_years` and `ceiling_years` are now in the stored `reasoning`, with
`none` distinguished from `0.0`, so a decision can be audited without re-parsing.

**The re-score could not reach the affected jobs.** `runScore` capped at
`List(ctx, 1000)` ordered by `discovered_at DESC`, and 7 of the 11 postings to be
vetoed were older than that. The note above justified the cap on the grounds that
overflow is "already scored", which only holds while the rules do not change.
Added `SQLiteRepository.Count`; `runScore` and `status` both size their `List` from
it. `status` was misreporting `Jobs (1000)` against 1347 stored while `tech.md`
points at it for real totals. `runScore`'s timeout went to 30 minutes: a deadline
mid-run leaves the table half old-rules and half new.

Measured on the live database, 1347 jobs, backed up first to
`data/backups/jobclaw.db.pre-experience-filter-20260909-170754`:

```
visible before   144 SHORTLIST/APPLY
visible after    140 SHORTLIST, 1207 SKIP
newly excluded    11  needs 3,3,3,3,4,4,4,4,5,8 years
newly included     7  previously suppressed by markup-degraded skill matching
```

Of the 140 visible, none states a requirement above two years: 111 state nothing,
12 state 2, 16 state 1, 1 states 0. The single posting still containing a ">2 years"
string anywhere is job 2135's "3 year vision".

Left open: 10 of the 11 newly-excluded jobs are already appended to the Google
Sheet and still read `SHORTLIST` there, because `sheet_synced_at` is set and status
only advances. `jobclaw mark <id> skipped` retires a row properly. Job 7 is
`APPLIED` and should stay.

Also note the excluded-role list is still load-bearing and not redundant with this:
the veto fires only on a printed figure, and "Senior"/"Staff"/"Principal" titles
frequently state none.

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
