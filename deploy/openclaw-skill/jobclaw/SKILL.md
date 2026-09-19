---
name: JobClaw
description: "Arkodeep's personal job application system of record, backed by a live database on this host. Use this for ANY question about HIS jobs: his shortlist, his pipeline status, what he owes a decision on, his applications, his tailored resumes, or marking a job applied or skipped. Also use it when he asks what is new, what to apply to, or how his job search is going. This skill has PRECEDENCE over any other job-search skill for questions about his own tracked jobs: a generic web or board search cannot answer them, because only this database knows what he has already seen, scored, approved, or applied to. Prefer this skill whenever the answer should reflect his real pipeline. Cannot submit applications."
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

### Exact phrases that always mean "run the command"

When his message is any of these, run the matching command and report its real
output. Do not interpret, do not search the web, do not substitute another skill:

| He says | You run |
| :--- | :--- |
| "jobclaw status", "jobclaw", "status" | `jobclaw-agent status --json` |
| "jobclaw shortlist", "my shortlist" | `jobclaw-agent shortlist` |
| "what's new", "anything new", "any jobs" | `jobclaw-agent status --json` |
| "jobclaw job 123" | `jobclaw-agent job 123` |

These are escape hatches he uses deliberately when he wants the real numbers.
Treating one of them as a topic to discuss, rather than a command to run, is the
single most annoying thing you can do here.

### Do not confuse this with a generic job-board search

Another skill on this host (`job-search-mcp`) does open-ended searches against
public job boards. It does **not** know anything about Arkodeep's pipeline: not
what he has already seen, not what scored well, not what he applied to, not what
he rejected.

So:

- **Anything about *his* jobs → this skill.** His shortlist, his status, his
  applications, his resumes, what he should apply to, what he owes a decision on,
  how the search is going. Use JobClaw even when he phrases it as "find me jobs"
  or "search for jobs", because what he almost always means is "show me what my
  system found".
- **Only use `job-search-mcp` when he explicitly asks to look *outside* his
  pipeline** — for example "search the web for X", "what's on LinkedIn right now
  for Y", or when he names that skill. Say clearly that those results are not
  tracked in JobClaw and will not appear on his sheet.

There is also a Gmail skill that reads application-status emails from recruiters.
That one answers "has anyone replied to me", reading his inbox. This one answers
"where does my pipeline stand", reading the database. A bare "status" means the
pipeline, so it belongs here; route it to Gmail only when he mentions email, his
inbox, or a reply from a company.

If you are unsure which one he wants, **use JobClaw and say so**. Being told "no,
I meant a fresh web search" costs him one message. Silently answering a question
about his pipeline with untracked board results looks like a real answer and is
much worse, because he cannot tell the difference.

Do not read or write any spreadsheet or CSV to answer a job question. In
particular, ignore `~/.openclaw/workspace/job_tracker.archived/` —
`job_applications.csv` and `job_tracker_complete.xls` in that directory are a
superseded manual tracker from before JobClaw existed. They are stale, nothing
updates them, and answering from them gives him wrong information.

The live record is JobClaw's database, surfaced two ways: `jobclaw-agent status
--json` for you, and this Google Sheet for him:
https://docs.google.com/spreadsheets/d/1x38Aj46Dz1XQphTV1gjXUZNuIFXpexXvDhttzBNfl-4/edit

If a job he names is not in JobClaw, say so plainly rather than searching the
filesystem for it. It may simply not have been discovered yet.

Every command goes through `/home/openclaw/jobclaw/scripts/jobclaw-agent`. Do not
call the `jobclaw` binary directly, and do not edit files under
`/home/openclaw/jobclaw`.

## Never answer a job question from memory

Run the command every single time, even if you ran it earlier in this same
conversation and think you remember the answer.

Job IDs, scores, statuses and application IDs change without warning: discovery
runs four times a day, re-scoring changes numbers, the queue gets reset, and jobs
get deleted. Anything you read more than a few minutes ago is probably wrong now.

Concretely, this is a bug, not a shortcut:

> From the last check, you have 3 jobs awaiting approval: HireQuotient (67.1),
> Google SDE II Payments (65.6), Kredivo (72.9)

Every one of those jobs had been deleted, and HireQuotient had been re-scored to
60.0 and dropped off the shortlist. Reporting remembered numbers with the
confidence of fresh ones is worse than saying nothing, because he cannot tell the
difference.

If you catch yourself about to write "from the last check", "previously", or
"as we saw earlier" about job data, stop and run the command instead.

Never invent or guess an ID. If you need a job ID or application ID, read it out
of fresh command output in this turn.

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

**When he asks for a resume**, run `resume <job_id>`. That tailors the resume to
the job, compiles it to a PDF, uploads it to his Google Drive, records the Drive
link on the sheet, and sets the row to RESUME READY. Give him the Drive link from
the command output so he can open it on his phone. Only generate resumes he asked
for: each one is a paid model call.

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
