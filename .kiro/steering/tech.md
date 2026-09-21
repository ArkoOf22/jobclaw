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
| `JOBCLAW_GREENHOUSE_BOARDS` | Comma/newline separated board tokens. Empty tokens are ignored; a token that 404s is skipped and reported without costing the other boards. |
| `JOBCLAW_GREENHOUSE_BASE_URL` | Enables the HTTP Greenhouse form provider when set. |
| `JOBCLAW_GREENHOUSE_API_KEY` | Greenhouse API credential. |
| `JOBCLAW_JOBSPY_URL` | JobSpy MCP server endpoint. |
| `JOBCLAW_JOBSPY_LIVE_TEST` | Set to `1` to run live JobSpy tests. |

Note: `.env.example` is currently empty and does not document any of the above.

## Config files

`config/candidate.yaml`, `config/preferences.yaml`, `config/resume.yaml`. The resume config is where
tailoring boundaries live — which transformations are permitted (rewording, reordering, skill
selection) and which are forbidden (metric changes, invented experience).

### Machine-readable state

```bash
jobclaw status          # human summary
jobclaw status --json   # stable contract for orchestrators
```

`status --json` is the interface an orchestrator such as OpenClaw should use.
Field names are snake_case and the shape is additive-only: new fields may appear,
existing ones will not change meaning. It reports job counts by status and
recommendation, application counts, the jobs awaiting a human approval decision,
and the applications that cannot progress alone, each with the next command.

**`awaiting_approval` is capped at 25, highest score first.** `awaiting_approval_total`
carries the true count, so a capped list is distinguishable from a short one. Use
`--limit N` to change the cap, or `--limit 0` for the full backlog.

The cap exists because this field used to return every pending row. At 2,150
pending jobs the document was 597KB, about 149,000 tokens, and the OpenClaw agent
reads `status --json` on most turns — so this one field was the single largest
token cost in the system and produced observed requests above 500,000 prompt
tokens. Capping it took the default payload to under 10KB, a 61x reduction. A
human cannot act on 2,150 rows anyway, so the full list was expensive and
unusable at the same time.

Applications with an unresolved submission attempt appear with an empty
`next_command`, because they must never be retried automatically.

Note `jobs list` shows only the first 20 rows. Use `status` for real totals.

### Scheduled discovery

`jobclaw-discover.timer` runs discovery, then scoring, then sheet sync every six
hours (randomized up to 20 minutes, `Persistent=true` so a missed window catches
up after downtime). It deliberately stops before approval: that is a human gate,
so nothing past it happens unattended.

```bash
sudo systemctl list-timers jobclaw-discover.timer
sudo systemctl start jobclaw-discover.service   # run one pass now
journalctl -u jobclaw-discover -n 50
```

Units are version-controlled in `deploy/`; the wrapper is
`scripts/scheduled-discovery.sh`, which rebuilds `bin/jobclaw` when any source
file is newer so a deploy cannot silently keep running stale code.

**The schedule is written in UTC.** `OnCalendar=*-*-* 00,06,12,18:30:00` lands on
00:00, 06:00, 12:00 and 18:00 IST, since the host runs UTC and IST is UTC+5:30.
Do not "simplify" the `:30` away.

There is no per-timer timezone option here: naming a timezone inside `OnCalendar`
needs systemd 252 and this host runs 249. The unit previously carried
`OnCalendarTimezone=Asia/Kolkata`, which is not a systemd directive at all. It was
silently ignored, so the documented IST schedule was really UTC and fired at
05:30/11:30/17:30/23:30 IST. `systemd-analyze verify` reports unknown keys like
this; run it after editing a unit.

**The unit is sandboxed, and the sandbox has to account for `gog`.**
`ProtectHome=read-only` plus `ProtectSystem=strict` means only the paths listed in
`ReadWritePaths` are writable. Sheet sync shells out to `gog`, which keeps its
OAuth refresh token in a file-backed keyring under the home directory and takes a
lock file to read it, so the keyring directories need to be writable too:

```
ReadWritePaths=/home/openclaw/jobclaw
ReadWritePaths=/home/openclaw/.local/share/gogcli
ReadWritePaths=/home/openclaw/.local/state/gogcli
ReadWritePaths=/home/openclaw/.config/gogcli
```

Without those, every scheduled sync failed with `read-only file system` on the
keyring lock while manual runs succeeded, because a manual run has a writable
home. That asymmetry is what made it hard to spot: the sheet fell behind for days
and the only evidence was in `journalctl`.

### Agent / Telegram control

OpenClaw drives JobClaw through `scripts/jobclaw-agent`, a restricted wrapper, not
the binary directly. The skill lives at `deploy/openclaw-skill/jobclaw/SKILL.md`
and is installed to `~/.openclaw/workspace/skills/jobclaw/`.

The wrapper enforces the submission gate structurally rather than relying on the
agent to follow instructions: `--confirm` is rejected in every form, `submit` is
always a dry run, arguments containing shell metacharacters are refused, and any
subcommand not explicitly allowlisted is refused. Adding a new JobClaw command
does not expose it to the agent until someone adds it here deliberately.

Allowed: `status`, `shortlist`, `jobs list`, `job`, `answers`, `approve`,
`reject`, `application`, `resume`, `questionnaire`, `prepare`, and `submit` as
preview. Refused: `discover`, `score`, `answer add/update`, and anything else.

Real submission stays with the human:

```bash
jobclaw submit <application_id> --confirm
```

### Pruning stale jobs

Discovery accumulates: a sweep stores everything that matched at the time, and
matching rules improve afterwards. Hundreds of stale rejects hide the real queue
and slow a full re-score.

```bash
jobclaw prune --older-than 30            # preview, nothing deleted
jobclaw prune --older-than 30 --confirm  # delete
```

Preview is the default, same gate as submission, because deletion is irreversible.

Never eligible: `SHORTLISTED` jobs, since those are open decisions; any job with
an application, since that implies generated artifacts and possibly a submission;
and anything at `APPROVED`, `APPLIED`, `INTERVIEW`, or `OFFER`. Asking to prune a
protected status is an error rather than a silently empty result. To retire a
shortlisted job, `reject` it first.

Each row is re-checked inside the delete transaction, so a job that gained an
application or advanced status between preview and confirm survives.

Back up `data/jobclaw.db` first. There is no undo.

### Google Sheet review list

```bash
jobclaw sheet init                 # write the header row, once
jobclaw sheet sync [--dry-run]     # append newly shortlisted jobs
jobclaw sheet rebuild [--confirm]  # empty it and write it again from current scores
```

### Rebuild when scoring rules change

The sheet is append-only and rows never revise themselves. `sheet sync` skips
anything already marked synced, so a row written as SHORTLIST keeps saying
SHORTLIST even after a rescore vetoes that job. Tightening the experience ceiling
left 168 such rows, including seven `Software Engineer III` roles — the candidate
reads the sheet on a phone, so the scorer change had not reached where they look.

`sheet rebuild` previews by default and only acts with `--confirm`, the same gate
as `submit` and `prune`. It snapshots the sheet to `data/backups/sheet-*.json`
first, clears the data rows, resets `sheet_synced_at`, then re-syncs in batches of
200 until the queue drains.

Order matters inside it: the sheet is cleared **before** the sync state is reset,
because resetting first and then failing to clear would duplicate every row.

**Anything past the approval gate stays on the sheet regardless of score.**
`ListUnsyncedForSheet` used to filter on recommendation alone, so a job that had
been applied to and then rescored to SKIP dropped off entirely — the record of
committed work erased by a scoring change. Rebuild also seeds the Status cell from
the job's real status rather than always writing `NEW`, for the same reason.

A rebuild does not restore Resume links; those are written by `jobclaw resume
<job_id>`, which is a paid model call, so they are not regenerated automatically.

Live sheet: `1x38Aj46Dz1XQphTV1gjXUZNuIFXpexXvDhttzBNfl-4`, tab `Jobs`.

Writes go through the `gog` CLI, which is already OAuth-authorised for
`arkodeepkoley123@gmail.com` with `spreadsheets` and `drive.file` scopes. No GCP
service account is involved.

Required in `.env`: `JOBCLAW_SHEET_ID`, `JOBCLAW_SHEET_ACCOUNT`,
`GOG_KEYRING_PASSWORD`. The last one is not optional: gog keeps its refresh token
in a file-backed keyring and cannot prompt for a password from a timer.

Optional: `JOBCLAW_SHEET_TAB` (default `Jobs`), `JOBCLAW_GOG_BIN` (default `gog`).

Setup notes, both of which cost time to rediscover:

- A new spreadsheet's only tab is `Sheet1`. Create the `Jobs` tab with
  `gog sheets add-tab <id> Jobs` or point `JOBCLAW_SHEET_TAB` at `Sheet1`.
- Scopes alone are not enough. The Sheets and Drive APIs must also be enabled in
  the Cloud project behind the OAuth client, otherwise calls fail with
  "Sheets API is not enabled for this OAuth project".

`--dry-run` needs no Google credentials at all, so row selection can be verified
without auth.

Sync is idempotent: `jobs.sheet_synced_at` is set only after a successful append,
so a second run adds nothing and a failed run leaves rows eligible for retry.

### Model selection

Two models, set in `config/resume.yaml` under `resume.llm`:

- `model` — resume tailoring. Prose a recruiter reads, so quality matters.
  Currently `google/gemini-2.5-flash`.
- `answers_model` — questionnaire answers. Short factual strings like "Yes" or
  "India". Currently `mistralai/mistral-small-24b-instruct-2501`. Falls back to
  `model` when unset.

Both are available on Zero Data Retention endpoints, so `deny_data_collection`
and `require_zero_data_retention` still hold.

Cost per resume, roughly 8k prompt and 1k completion tokens:

| Model | Per resume | At 300/month |
| :--- | :--- | :--- |
| `anthropic/claude-sonnet-4.5` (previous) | $0.0390 | $11.70 |
| `google/gemini-2.5-flash` (current) | $0.0049 | $1.47 |
| `mistralai/mistral-small-24b-instruct-2501` | $0.0005 | $0.14 |

A/B on job 473 found Gemini Flash equal or better: identical fact preservation
across every checked term, and 2563 bytes against Sonnet's 3384 for the same
content. Sonnet padded with a generic summary paragraph; Gemini kept the
quantified bullets and dropped the filler. Being closer to the 2648-byte master
is the desirable direction.

Do not chase the OpenRouter discount list. Discounts rotate, so a pinned
discounted model breaks silently when one ends, and cheap models are frequently
cheap because they train on inputs, which the ZDR constraint excludes anyway.

The larger cost lever is prompt size, not model choice: the full job description
is sent, and input is roughly 60% of each resume's cost. Trimming it would save
more than any model swap, with no quality risk.

### Prompt cost: what worked and what did not

Job descriptions are stored as **escaped HTML**, so a single `<` occupies `&lt;`
and a non-breaking space eleven characters. `NormalizeJobDescription` unescapes
twice, strips tags, and collapses whitespace. That is a pure saving of roughly
13%: no information is lost and the model gets a cleaner prompt. It is always
applied.

`TrimJobDescription` additionally drops employer boilerplate sections ("About
Stripe", "Who we are", benefits, EEO). It is implemented and tested but **not used
for resume prompts**, for two measured reasons:

- **The saving is negligible.** Against `gemini-2.5-flash` it cuts about $0.0003
  per resume, roughly $0.10 a month. Output tokens are 72% of the cost, so input
  trimming barely moves the total. The earlier claim that input was 60% of cost
  was true only at claude-sonnet-4.5's $15/M output pricing; switching model
  invalidated it.
- **It measurably reduced keyword density.** Regenerating job 473 with boilerplate
  dropped produced Go 3 times against 5 in the master, Kafka 2 against 3, Redis 3
  against 4. Restoring full context brought every count back to parity. For a
  document screened by ATS keyword matching, that is the wrong trade for $0.10.

`capJobDescription` bounds pathological descriptions at 6000 characters, cutting
on a paragraph boundary so a requirement is never severed.

The remaining cost lever is **output** tokens, not input. Shorter resumes would be
cheaper, but resume length is a quality decision, not a cost one.

### Closing the loop: mark

```bash
jobclaw mark <job_id> applied   # applied by hand
jobclaw mark <job_id> skipped   # decided against it
```

Submission happens on the employer's own form, so nothing can detect it. The
candidate reports it and this records it, updating the database and the sheet's
Status cell together. Without it the sheet fills with rows still marked NEW.

`applied` bridges through APPROVED when needed, since applying to a job is the
approval. `skipped` rejects it so it never resurfaces.

`resume <job_id>` also writes the artifact path into the sheet's Resume cell and
sets the row to RESUME READY.

Sheet writes here are best-effort: the database is the source of truth, so a
Google API failure reports itself and leaves the state change standing rather than
rolling it back. Cells are written individually because Resume (I) and Status (J)
are not adjacent to the columns between them, and a range write would clobber Why.

Rows are located by scanning column A for the job ID, since the sheet is written
in score order and rows are never renumbered.
