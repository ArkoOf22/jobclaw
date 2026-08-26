---
name: JobClaw
description: The job application system of record. Use for ANY request about jobs, job search, applications, shortlists, tailored resumes, or marking a job applied or skipped. Surfaces shortlisted roles, prepares applications, and records outcomes. Cannot submit applications.
slug: jobclaw
tags:
  - job-search
  - career
  - applications
  - resume
  - jobclaw
---

# JobClaw

JobClaw is an approval-gated job discovery and application system running on this
host. This skill lets you operate it from a chat channel on Arkodeep's behalf.

## Use this skill for anything job-related

**JobClaw is the source of truth for jobs and applications.** If Arkodeep mentions
a job, a company he is applying to, a shortlist, a resume, or says he applied to
something, use the commands below.

Do not read or write any spreadsheet or CSV to answer a job question. In
particular, ignore `~/.openclaw/workspace/job_tracker/` — `job_applications.csv`
and `job_tracker_complete.xls` in that directory are a superseded manual tracker
from before JobClaw existed. They are stale, they are not updated by anything, and
answering from them gives him wrong information.

The live record is JobClaw's database, surfaced two ways: `jobclaw-agent status
--json` for you, and this Google Sheet for him:
https://docs.google.com/spreadsheets/d/1x38Aj46Dz1XQphTV1gjXUZNuIFXpexXvDhttzBNfl-4/edit

If a job he names is not in JobClaw, say so plainly rather than searching the
filesystem for it. It may simply not have been discovered yet.

Every command goes through `/home/openclaw/jobclaw/scripts/jobclaw-agent`. Do not
call the `jobclaw` binary directly, and do not edit files under
`/home/openclaw/jobclaw`.

## The one rule

**You cannot submit an application, and you must not try.**

Submitting is irreversible and externally visible: it sends Arkodeep's real
application to a real employer. `jobclaw-agent submit <id>` is always a dry run
and the wrapper rejects `--confirm`. When an application is ready, deliver the
preview and tell him to run the real submission himself:

```
jobclaw submit <application_id> --confirm
```

Never suggest a way around this. Never offer to do it for him.

## What you can do

```bash
jobclaw-agent status --json          # whole pipeline as JSON, start here
jobclaw-agent status                 # same, human-readable
jobclaw-agent shortlist              # jobs recommended for application
jobclaw-agent job <job_id>           # one job with score breakdown
jobclaw-agent jobs list              # first 20 jobs only, prefer status
jobclaw-agent answers                # verified answer bank

jobclaw-agent approve <job_id>       # after he says yes
jobclaw-agent reject <job_id>        # after he says no

jobclaw-agent application <job_id>   # create workspace, tailor resume
jobclaw-agent resume <job_id>        # retry resume generation
jobclaw-agent questionnaire <app_id> --from-greenhouse
jobclaw-agent questionnaire <app_id> # re-resolve answers only
jobclaw-agent prepare <app_id>       # validate readiness
jobclaw-agent submit <app_id>        # PREVIEW ONLY

jobclaw-agent sheet sync             # add new shortlisted jobs to the sheet

jobclaw-agent mark <job_id> applied  # he applied by hand
jobclaw-agent mark <job_id> skipped  # he decided against it
```

`mark` closes the loop. Submission happens by hand on the employer's own form, so
nothing can detect it: he has to tell you, and you record it. Without that the
sheet fills with rows still marked NEW.

`mark applied` also advances the job through APPROVED if needed, because applying
to something is the approval. `mark skipped` rejects it so it never resurfaces.

The review sheet is the shared source of record Arkodeep reads on his phone:
https://docs.google.com/spreadsheets/d/1x38Aj46Dz1XQphTV1gjXUZNuIFXpexXvDhttzBNfl-4/edit

Row numbers in the sheet are not job IDs. The first column is the job ID; use that
when calling commands, and refer to jobs by company and title when talking to him.

Anything not listed is refused by the wrapper.

## Reading status

`status --json` is the contract. Use it rather than parsing human output.

- `jobs.by_status`, `jobs.by_recommendation`, `applications.by_status` are counts.
- `awaiting_approval` is what Arkodeep owes a decision on. Each entry has
  `job_id`, `company`, `title`, `location`, `url`, `source`, `score`,
  `recommendation`.
- `needs_attention` is approved work that cannot progress alone. Each entry has
  `application_id`, `reason`, and `next_command`.

An entry in `needs_attention` with an **empty `next_command`** means a submission
attempt could not be confirmed. Do not act on it and do not retry anything.
Report it and say it needs manual reconciliation.

## Flow

**When he asks what's new**, run `status --json` and report the counts plus any
`SHORTLIST` entries in `awaiting_approval`. Lead with the shortlisted roles: those
are the decisions he owes. Give company, title, location, score, and the URL.
Ignore `SKIP` entries unless he asks.

**When he approves a job**, run `approve <job_id>`, then
`application <job_id>` to create the workspace and tailor the resume, then
`questionnaire <app_id> --from-greenhouse` to fetch the real form and resolve
answers, then `prepare <app_id>`.

Note that `application` takes a **job** id and everything after it takes an
**application** id. Read the application id from the `application` output or from
`status --json`.

**If `prepare` reports BLOCKED**, tell him exactly which blockers came back. The
usual cause is a question with no verified answer. He fixes that with
`jobclaw answer add <field_key> <answer>` himself, then you re-run
`questionnaire <app_id>` and `prepare <app_id>`.

**When he says he applied**, run `mark <job_id> applied`. When he passes on
something, run `mark <job_id> skipped`. Confirm which job you acted on by company
and title, not by row number, so a mistake is visible before it matters.

**When he asks for a resume**, run `resume <job_id>`. That generates the tailored
resume, records its path on the sheet, and sets the row to RESUME READY. Tell him
the file path. Only generate resumes he asked for: each one is a paid model call.

**When `prepare` reports READY**, run `submit <app_id>` to get the preview and
send him the whole thing: target employer, resume path, and every question with
its answer and source. Then give him the `--confirm` command to run himself.

## Things to flag, not hide

- Answers marked `source=LLM` are generated, not verified by Arkodeep. Call them
  out explicitly in the review packet. He should read those before submitting.
- The `answer_provenance` readiness check lists them and does not block.
- If a question's answer looks wrong for his situation, say so. A known example:
  US-only questions can pick up his Indian address.

## What you must not do

- Never run the real submission, or tell him how to bypass the gate.
- Never invent an answer to an application question. If the answer bank has no
  verified answer, that is a blocker for him to resolve, not a gap for you to
  fill.
- Never approve or reject a job on your own initiative. Approval is his decision.
- Never quote his API keys, or the contents of `.env`.
- Do not mass-approve. JobClaw is a precision tool, not a bulk applier.

## Scheduling

Discovery and scoring run automatically every six hours via
`jobclaw-discover.timer`, so new roles appear without being asked. You do not
need to trigger discovery.
