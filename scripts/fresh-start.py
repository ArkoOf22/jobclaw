#!/usr/bin/env python3
"""Clear the review queue so discovery can start from a clean slate.

Why this exists
---------------
The queue accumulates. Jobs scored weeks ago sit alongside today's, the sheet
grows rows nobody will action, and the list stops being a thing you can read top
to bottom. Periodically you want to declare a fresh start.

What it deliberately keeps
--------------------------
candidate_answers
    Verified answers to employer questions, built up by hand. Expensive in the
    only currency that matters here: the candidate's attention. Never cleared.

companies
    Classification results from the LLM. Rebuilding costs real money, and a
    company's nature does not change because the queue was reset.

jobs with status APPLIED, and their applications
    Applications actually submitted are history, not queue. Deleting them would
    make the tool forget what the candidate did.

job_sources
    Configuration.

Everything else in the job pipeline goes.

Default is a dry run. Pass --confirm to write.
"""

import argparse
import sqlite3
import sys

# Jobs in this state are history and survive a reset.
KEEP_JOB_STATUS = "APPLIED"

# Ordered child-to-parent so no delete strands a foreign key.
DELETES = (
    (
        "application_events",
        """DELETE FROM application_events
           WHERE application_id IN (
             SELECT id FROM applications WHERE job_id IN (
               SELECT id FROM jobs WHERE status <> ?
             )
           )""",
    ),
    (
        "application_questions",
        """DELETE FROM application_questions
           WHERE application_id IN (
             SELECT id FROM applications WHERE job_id IN (
               SELECT id FROM jobs WHERE status <> ?
             )
           )""",
    ),
    (
        "applications",
        """DELETE FROM applications
           WHERE job_id IN (SELECT id FROM jobs WHERE status <> ?)""",
    ),
    (
        "job_scores",
        """DELETE FROM job_scores
           WHERE job_id IN (SELECT id FROM jobs WHERE status <> ?)""",
    ),
    (
        "jobs",
        "DELETE FROM jobs WHERE status <> ?",
    ),
)

PRESERVED = ("candidate_answers", "companies", "job_sources")


def counts(conn, table, where="", args=()):
    sql = "SELECT COUNT(*) FROM %s %s" % (table, where)

    return conn.execute(sql, args).fetchone()[0]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", default="data/jobclaw.db")
    parser.add_argument(
        "--confirm",
        action="store_true",
        help="actually delete; omit for a dry run",
    )
    opts = parser.parse_args()

    conn = sqlite3.connect(opts.db)
    conn.execute("PRAGMA foreign_keys = ON")

    print("JobClaw Fresh Start")
    print("-" * 46)
    print("Database: %s" % opts.db)
    print("Mode:     %s" % ("WRITE" if opts.confirm else "dry run"))
    print()

    print("Clearing:")

    planned = []

    for table, sql in DELETES:
        # Count via the same predicate the delete uses, so the preview cannot
        # drift from what actually happens.
        preview = sql.replace(
            "DELETE FROM %s" % table,
            "SELECT COUNT(*) FROM %s" % table,
            1,
        )
        n = conn.execute(preview, (KEEP_JOB_STATUS,)).fetchone()[0]
        planned.append((table, sql, n))
        print("  %-24s %d" % (table, n))

    print()
    print("Keeping:")
    print(
        "  %-24s %d"
        % (
            "jobs (%s)" % KEEP_JOB_STATUS,
            counts(conn, "jobs", "WHERE status = ?", (KEEP_JOB_STATUS,)),
        )
    )
    print(
        "  %-24s %d"
        % (
            "applications",
            counts(
                conn,
                "applications",
                "WHERE job_id IN (SELECT id FROM jobs WHERE status = ?)",
                (KEEP_JOB_STATUS,),
            ),
        )
    )

    for table in PRESERVED:
        print("  %-24s %d" % (table, counts(conn, table)))

    if not opts.confirm:
        print()
        print("Dry run. Re-run with --confirm to apply.")

        return 0

    print()

    with conn:
        for table, sql, _ in planned:
            conn.execute(sql, (KEEP_JOB_STATUS,))

    conn.execute("VACUUM")

    print("Done. Remaining:")

    for table in ("jobs", "job_scores", "applications") + PRESERVED:
        print("  %-24s %d" % (table, counts(conn, table)))

    conn.close()

    return 0


if __name__ == "__main__":
    sys.exit(main())
