# Google Sheet — Status dropdown (one-time manual setup)

The review sheet's **Status** column (column **J**, header "Status") is where you
track what you did with each job. Instead of typing `applied` / `skipped` by
hand, you can turn that column into a click-to-pick dropdown. This is a one-time
setup you do in the Sheet UI; after that it applies to every row, including rows
JobClaw appends later.

## Why this is manual, not automated

JobClaw writes to the sheet through the `gog` CLI. `gog sheets` exposes value
operations (`get`, `update`, `append`, `batch-update`) but **not** the Google
Sheets `setDataValidation` request, which is the only API that creates a
dropdown. So the dropdown cannot be created from code with the current tooling.
Setting it once in the UI is the honest, working path — it is not a placeholder
for missing work.

Verified against `gog v0.36.0`: `batch-update --data-json` takes value ranges
only, so it cannot carry a `setDataValidation` request.

## One-time setup

1. Open the sheet, tab **Jobs**.
2. Select the whole **Status** column: click the **J** column header. (Selecting
   the column, not a range, is what makes the rule apply to future rows too.)
3. Menu: **Data → Data validation → Add rule**.
4. Criteria: **Dropdown**. Add these values, one per line — they must match
   exactly what JobClaw writes, or an automated write will show as an "invalid"
   value:

   | Value | Who sets it | Meaning |
   | :--- | :--- | :--- |
   | `NEW` | JobClaw (`sheet sync`) | Newly appended, no decision yet |
   | `SHORTLIST` | you | You want to consider it |
   | `APPROVED` | you | You have decided to apply |
   | `RESUME READY` | JobClaw (`resume <id>`) | Tailored resume generated |
   | `APPLIED` | JobClaw (`mark <id> applied`) | Applied by hand |
   | `SKIPPED` | JobClaw (`mark <id> skipped`) | Decided against it |

5. Leave "Show a warning" (not "Reject the input") so JobClaw's automated writes
   are never blocked if the value list and the code ever drift.
6. Save.

The exact strings JobClaw writes are defined in `cmd/jobclaw/mark.go`
(`sheetStatusApplied`, `sheetStatusSkipped`, `sheetStatusResumeReady`) and
`cmd/jobclaw/sheet.go` (`"NEW"`). Keep this list in sync if those change.

## Important: the sheet is one-way

This is the limitation to be clear-eyed about. **JobClaw writes to the sheet but
never reads it back.** The SQLite database is the source of truth.

So if you pick `APPLIED` from the dropdown, the *sheet cell* changes but the
*database does not*. To actually record the state — which is what drives
`status`, the orchestrator contract, and future runs — you still run:

```bash
jobclaw mark <job_id> applied     # or: skipped
```

That command updates the database **and** writes the same value back to the sheet
cell, so the two stay consistent. The dropdown is a convenience for eyeballing
and quick manual notes; it is not a control surface that feeds back into JobClaw.

Closing that loop (reading edits back from the sheet into the database) would be
a separate feature: it needs a read-and-reconcile path and a rule for who wins
when the sheet and the database disagree. It is deliberately not part of this
change.
