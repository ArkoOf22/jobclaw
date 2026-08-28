# JobClaw

> An approval-gated job discovery and application system. It finds relevant jobs, scores them, prepares tailored resumes, and tracks what you applied to — while leaving every consequential decision to you.

JobClaw automates the repetitive half of a job search (searching, filtering, scoring, resume tailoring) and stops at a hard human gate before anything consequential happens. It never applies on your behalf, and it never invents facts about you.

It runs as a Go service on a small EC2 host, discovers jobs on a schedule, publishes a shortlist to a Google Sheet you read on your phone, and is driven day to day through a Telegram bot.

---

## The model: two halves, one wall

```text
   AUTOMATIC (every 6 hours)                 YOU DECIDE (via Telegram / sheet)
   ─────────────────────────────            ───────────────────────────────────
   discover → score → sheet sync    ──▶      review → resume → apply by hand → mark
```

Everything up to a shortlist happens on its own. Everything after it — spending a paid model call on a resume, applying, recording an outcome — needs you. The wall is the `APPROVED` boundary between a *job* and an *application*, and it is load-bearing.

---

## How you use it (Telegram)

You talk to the bot in plain English. It recognises anything job-related and runs JobClaw behind the scenes through a restricted wrapper — you never type raw commands.

| You say | What happens |
| :--- | :--- |
| "what's new" / "show my shortlist" | Reads the live shortlist: company, title, location, score |
| "make me a resume for the Stripe role" | Tailors a resume, compiles a PDF, uploads it to Google Drive, puts the link on the sheet |
| "I applied to Stripe" | Marks that job applied so it stops resurfacing |
| "skip the Kredivo one" | Rejects it |
| "prepare the GitLab application" | Fetches the form's questions and answers them from your verified answer bank |

Your daily loop:

1. The sheet fills with new roles automatically (no action from you).
2. On Telegram: **"what's new"** → glance at the top few.
3. **"make me a resume for [company]"** → get the Drive link.
4. Open the job link, apply by hand using that PDF.
5. **"applied to [company]"** → done, it won't nag you again.

The bot reads live data on every request, so figures are always current.

---

## The one thing it will never do: auto-submit

**JobClaw does not submit applications, and this is permanent.** Every major ATS blocks programmatic submission — Greenhouse with reCAPTCHA Enterprise plus a per-request token, Lever with hCaptcha, Ashby with reCAPTCHA and explicit anti-automation flags, Workday with per-employer accounts. This was investigated empirically; it is an industry norm, not a gap in JobClaw.

So the terminal step is always a human one: JobClaw hands you a tailored resume and the apply URL, you submit on the employer's own form, then you tell the bot "applied." A Greenhouse submission adapter exists behind the platform boundary, but the realistic and permanent adapter is `MANUAL`.

---

## What runs automatically

A systemd timer (`jobclaw-discover.timer`) fires four times a day (00/06/12/18 IST, 72-hour window). Each run does **discover → score → sheet sync** and then stops. It never crosses the approval wall.

### Discovery

Sources run concurrently behind a common `Source` interface; one failing source never stops the others.

- **JobSpy** (MCP server on loopback) scrapes LinkedIn, Indeed, Glassdoor, and Naukri. Each target role is sent as its **own clean query** (not concatenated), Indeed is pinned to India, and results are relevance-filtered against the shared matcher.
- **Greenhouse** public Job Board API across ~12 company boards (Stripe, PhonePe, GitLab, Twilio, Airbnb, Postman, CockroachLabs, Druva, Slice, Wise, MongoDB, Zscaler). No credentials needed for reads.

### Scoring

Each job gets component scores (skills, role, experience, domain, location, company, compensation) normalised over the signals actually present, so a source that never reports salary is not penalised for it. On top of the score sit **hard vetoes** that a strong score cannot override:

- **Experience** — a posting explicitly demanding more than *candidate years + 2* is skipped.
- **Location** — a foreign-country signal with no India signal, or a bare "remote" with no India mention, is skipped.
- **Excluded titles** — Senior / Sr / Lead / Staff / Principal / Manager and non-permanent roles (intern, trainee) are skipped.

Jobs at or above the shortlist threshold (currently 60/100) are synced to the sheet.

### The review sheet

The shortlist is written to a Google Sheet — sortable, readable on the mobile app — via the `gog` CLI (already OAuth-authorised, no GCP service account). Columns include job ID, score, verdict, company, title, location, URL, status, and the resume link. SQLite remains the source of truth; the sheet is a pure view, and rows are appended only after a successful write.

---

## Resume tailoring → PDF → Drive

Asking for a resume runs a single pipeline:

```text
master resume + job  →  LLM (structured JSON, fact-guarded)  →  LaTeX template  →  pdflatex  →  PDF  →  Google Drive  →  link on sheet
```

- The model returns the tailored resume as **structured JSON drawn strictly from your master resume**. The job description is a relevance signal only, never evidence of a skill.
- The output is **fact-guarded**: every metric, company, date, and technology must trace back to the master resume, or it is rejected and regenerated.
- Contact details and education come from `config/candidate.yaml` and never pass through the model.
- The JSON is rendered into a fixed LaTeX template and compiled with `pdflatex` (chosen over XeTeX-based engines because the template uses pdfTeX features for clean ATS text extraction).
- The PDF is uploaded to a "JobClaw Resumes" Drive folder; the link lands on the sheet so you can open it from your phone.

Models are pinned in `config/resume.yaml`: `google/gemini-2.5-flash` for resume prose, `mistralai/mistral-small-24b-instruct-2501` for questionnaire answers. Both run on Zero-Data-Retention endpoints, enforced per request.

---

## Questionnaires and the answer bank

For forms with application questions (e.g. Greenhouse), JobClaw fetches the questions from the public API, classifies each field, and resolves it against a **verified answer bank** — a store of candidate facts the operator has confirmed. Unresolved or LLM-generated answers are surfaced for review rather than guessed, and readiness validation blocks a preparation that cannot confirm its questionnaire was ingested, its answers are non-empty, and its resume file actually exists.

---

## Repository layout

```text
jobclaw/
├── cmd/jobclaw/main.go            # CLI composition root — wires everything
├── internal/
│   ├── discovery/                 # source abstraction, concurrent service, shared matcher
│   │   ├── jobspy/                # JobSpy MCP client (per-role search, India-pinned)
│   │   └── greenhouse/            # public board API client, multi-board
│   ├── job/                       # job domain, status transitions, SQLite repo
│   ├── scoring/                   # component scores + hard vetoes
│   ├── company/                   # evidence-based company classification
│   ├── application/               # the largest package: application lifecycle,
│   │                              #   resume generation (text + LaTeX/PDF),
│   │                              #   questionnaire ingestion, answer bank,
│   │                              #   preparation/readiness, submission adapters
│   ├── drive/                     # gog-backed Google Drive uploader
│   ├── sheet/                     # gog-backed Google Sheet writer
│   ├── llm/openrouter/            # LLM client (ZDR-enforced)
│   ├── config/                    # YAML config models
│   └── database/                  # SQLite connection + migrations
├── config/                        # candidate.yaml, preferences.yaml, resume.yaml
├── deploy/                        # systemd units, OpenClaw skill
└── scripts/                       # scheduled-discovery.sh, jobclaw-agent wrapper, helpers
```

Platform-specific complexity (Greenhouse URLs, forms, submission) lives behind platform boundaries; the core application workflow stays platform-independent, and a new ATS plugs in at the same seam.

---

## CLI

`cmd/jobclaw/main.go` dispatches:

```text
discover [--hours N] [--limit N]     score        shortlist        status [--json]
jobs list        job <id>            approve <id>     reject <id>
application <jobID>                  resume <jobID>
answer add|list|update               questionnaire <appID> [--from-greenhouse]
prepare <appID>                      submit <appID> [--confirm]
sheet init|sync [--dry-run]          mark <jobID> applied|skipped
prune [--older-than N] [--confirm]
```

`submit` is a dry run by default and only sends with `--confirm`. Exit code `2` means the outcome is **ambiguous** — the application stays locked and is never auto-retried.

### The agent wrapper

Telegram control goes through `scripts/jobclaw-agent`, a restricted allowlist, not the binary directly. It rejects `--confirm` in every form, refuses shell metacharacters, and passes through only safe subcommands (`status`, `shortlist`, `jobs list`, `job`, `answers`, `approve`, `reject`, `application`, `resume`, `questionnaire`, `prepare`, `mark`, `sheet sync`, and `submit` as preview). `discover`, `score`, and `answer add/update` are deliberately not exposed. Adding a new command to the binary does not expose it to the agent until someone adds it here.

---

## Setup and operations

### Runtime

- **Build/run host:** EC2 (Ubuntu 22.04), Go 1.26.5. The local macOS checkout is for editing only (Go 1.25.5 cannot build). Access is over SSM Session Manager.
- **Dependencies:** SQLite via `modernc.org/sqlite` (pure Go, no cgo); `pdflatex` (TeX Live) for resume PDFs; the `gog` CLI for Sheets/Drive; the JobSpy MCP server on `127.0.0.1:8000`.

### Configuration

Behaviour is driven by three YAML files in `config/`:

- `candidate.yaml` — identity, contact (for the resume header), target roles, skills, experience, education.
- `preferences.yaml` — locations, excluded titles, domains, tech preferences, and the shortlist threshold.
- `resume.yaml` — master resume path, tailoring rules, and pinned models.

### Environment (`.env` on the host, mode 600)

| Variable | Purpose |
| :--- | :--- |
| `OPENROUTER_API_KEY` | LLM key (name is itself configurable via `resume.yaml`) |
| `JOBCLAW_GREENHOUSE_BOARDS` | Comma-separated Greenhouse board tokens |
| `JOBCLAW_SHEET_ID` | Review sheet spreadsheet ID |
| `JOBCLAW_SHEET_ACCOUNT` | Google account `gog` acts as |
| `JOBCLAW_RESUME_DRIVE_FOLDER` | Drive folder ID for uploaded resumes |
| `GOG_KEYRING_PASSWORD` | Unlocks `gog`'s credential keyring (no terminal to prompt from a timer) |

Optional: `JOBCLAW_SHEET_TAB`, `JOBCLAW_GOG_BIN`, `JOBCLAW_PDFLATEX_BIN`, `JOBCLAW_DB_PATH`, `JOBCLAW_JOBSPY_URL`, `JOBCLAW_GREENHOUSE_BASE_URL`.

### Scheduled discovery

```bash
sudo systemctl list-timers jobclaw-discover.timer   # next run
sudo systemctl start jobclaw-discover.service        # run one pass now
journalctl -u jobclaw-discover -n 50                 # logs
```

The wrapper rebuilds the binary when source is newer, so a deploy cannot silently keep running stale code.

### Build and test (on the EC2 host)

```bash
go build ./...
go vet ./...
go test ./...
```

---

## Design principles

- **Human approval before consequential actions.** No workspace assets are generated and no submission executes without explicit consent. The `APPROVED` boundary is a hard wall.
- **Candidate truth.** Tailoring may reword, reorder, and select; it may not invent experience, metrics, or history. Enforced by a fact guard, not just a prompt.
- **Preparation ≠ execution.** Prepared data is inspectable and validatable before any side effect.
- **Ambiguity is a real state.** An unknown external outcome stays locked for manual reconciliation; it is never silently converted to success or failure, and never auto-retried.
- **Source isolation.** One failing discovery provider does not stop the others.
- **Cost discipline.** Cheap ZDR-compliant models, one model call per resume (feeding both PDF and text), and no speculative generation.

---

## Status

Operational. Discovery, scoring, the review sheet, the resume-to-PDF-to-Drive pipeline, questionnaire ingestion, and the Telegram control loop all run against real data on the live host. The build, vet, and full test suite pass.

The submission step is, and will remain, a one-tap human action for the reasons above. Future work is additive: more discovery sources and Greenhouse-style read/form integrations for other ATSes (Lever, Ashby both expose clean public read APIs), richer company intelligence, and operational reporting.
