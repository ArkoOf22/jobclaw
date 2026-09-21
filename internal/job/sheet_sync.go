package job

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SheetCandidate is a job that should appear on the review sheet, joined with its
// latest score so the sheet can show the verdict without a second lookup.
type SheetCandidate struct {
	Job            Job
	OverallScore   float64
	Recommendation string
	Reasoning      string
}

// ListUnsyncedForSheet returns scored jobs that belong on the review sheet and
// have not been written yet.
//
// Only SHORTLIST and APPLY are included. The sheet is a worklist, so filling it
// with everything discovered would defeat the point: the value is that scoring
// already discarded the rest.
//
// The exception is anything past the approval gate. A job that was approved or
// applied to belongs on the sheet whatever it now scores, because the sheet is
// also how the candidate tracks what they have already done. Without this a
// rescore under tighter rules would quietly erase applied jobs from their record
// — which is exactly what happened when the experience ceiling was tightened and
// two applied roles dropped to SKIP.
func (r *SQLiteRepository) ListUnsyncedForSheet(
	ctx context.Context,
	limit int,
) ([]SheetCandidate, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			j.id,
			j.status,
			j.external_id,
			js.name,
			j.company,
			j.title,
			COALESCE(j.location, ''),
			j.url,
			j.discovered_at,
			s.overall_score,
			s.recommendation,
			COALESCE(s.reasoning, '')
		FROM jobs j
		JOIN job_sources js ON js.id = j.source_id
		JOIN job_scores s ON s.job_id = j.id
		WHERE j.sheet_synced_at IS NULL
		  AND j.status NOT IN ('REJECTED')
		  AND (
			s.recommendation IN ('SHORTLIST', 'APPLY')
			-- Work already committed to stays on the sheet regardless of score.
			OR j.status IN ('APPROVED', 'APPLIED', 'INTERVIEW', 'OFFER')
		  )
		  -- Only the newest score per job, since scoring re-runs append.
		  AND s.id = (
			SELECT MAX(s2.id) FROM job_scores s2 WHERE s2.job_id = j.id
		  )
		ORDER BY s.overall_score DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list unsynced sheet jobs: %w", err)
	}
	defer rows.Close()

	candidates := make([]SheetCandidate, 0)

	for rows.Next() {
		var (
			candidate    SheetCandidate
			status       string
			location     sql.NullString
			discoveredAt string
		)

		if err := rows.Scan(
			&candidate.Job.ID,
			&status,
			&candidate.Job.ExternalID,
			&candidate.Job.Source,
			&candidate.Job.Company,
			&candidate.Job.Title,
			&location,
			&candidate.Job.URL,
			&discoveredAt,
			&candidate.OverallScore,
			&candidate.Recommendation,
			&candidate.Reasoning,
		); err != nil {
			return nil, fmt.Errorf("scan sheet candidate: %w", err)
		}

		candidate.Job.Status = Status(status)

		if location.Valid {
			candidate.Job.Location = location.String
		}

		parsed, err := parseTime(discoveredAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse discovered_at for job %d: %w",
				candidate.Job.ID,
				err,
			)
		}

		candidate.Job.DiscoveredAt = parsed
		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sheet candidates: %w", err)
	}

	return candidates, nil
}

// CountSheetEligible reports how many jobs belong on the sheet under current
// scores, regardless of whether they have been written yet.
//
// Used by the rebuild to state up front how many rows it will produce, so the
// operation is predictable before it clears anything.
func (r *SQLiteRepository) CountSheetEligible(ctx context.Context) (int, error) {
	var count int

	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM jobs j
		JOIN job_scores s ON s.job_id = j.id
		WHERE j.status NOT IN ('REJECTED')
		  AND (
			s.recommendation IN ('SHORTLIST', 'APPLY')
			OR j.status IN ('APPROVED', 'APPLIED', 'INTERVIEW', 'OFFER')
		  )
		  AND s.id = (
			SELECT MAX(s2.id) FROM job_scores s2 WHERE s2.job_id = j.id
		  )
	`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sheet-eligible jobs: %w", err)
	}

	return count, nil
}

// ListSheetStranded returns jobs past the approval gate that a rebuild still
// cannot place on the sheet.
//
// Normally empty: ListUnsyncedForSheet deliberately keeps approved and applied
// work regardless of score, so a rescore cannot erase it. What remains here is
// the genuinely unscorable — a job past the gate with no score row at all, which
// the sheet has no verdict to write for. Reported rather than silently skipped,
// because an applied job missing from the record is the kind of gap that is only
// noticed much later.
func (r *SQLiteRepository) ListSheetStranded(
	ctx context.Context,
) ([]SheetCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			j.id,
			j.status,
			j.company,
			j.title,
			COALESCE(s.overall_score, 0),
			COALESCE(s.recommendation, '')
		FROM jobs j
		LEFT JOIN job_scores s ON s.id = (
			SELECT MAX(s2.id) FROM job_scores s2 WHERE s2.job_id = j.id
		)
		WHERE j.status IN ('APPROVED', 'APPLIED', 'INTERVIEW', 'OFFER')
		  AND s.id IS NULL
		ORDER BY j.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list stranded sheet jobs: %w", err)
	}
	defer rows.Close()

	stranded := make([]SheetCandidate, 0)

	for rows.Next() {
		var (
			candidate SheetCandidate
			status    string
		)

		if err := rows.Scan(
			&candidate.Job.ID,
			&status,
			&candidate.Job.Company,
			&candidate.Job.Title,
			&candidate.OverallScore,
			&candidate.Recommendation,
		); err != nil {
			return nil, fmt.Errorf("scan stranded job: %w", err)
		}

		candidate.Job.Status = Status(status)

		stranded = append(stranded, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stranded jobs: %w", err)
	}

	return stranded, nil
}

// ResetSheetSync clears the record of what has been written to the sheet, making
// every qualifying job eligible again. Returns how many rows were reset.
//
// Only meaningful alongside emptying the sheet itself. The sheet is an output and
// this column is the only memory of what was already sent, so resetting it
// without clearing the sheet would duplicate every row.
//
// Needed because a row's verdict can change after it was written — rescoring
// under tighter rules turns a SHORTLIST into a SKIP — and nothing rewrites a row
// that is already marked synced. Left alone, the sheet keeps advertising jobs the
// scorer has since rejected.
func (r *SQLiteRepository) ResetSheetSync(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE jobs
		SET sheet_synced_at = NULL
		WHERE sheet_synced_at IS NOT NULL
	`)
	if err != nil {
		return 0, fmt.Errorf("reset sheet sync state: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count reset rows: %w", err)
	}

	return affected, nil
}

// MarkSheetSynced records that these jobs have been written to the sheet.
//
// Called only after the write succeeds, so a failed append leaves them eligible
// for the next run rather than silently dropping them.
func (r *SQLiteRepository) MarkSheetSynced(
	ctx context.Context,
	ids []int64,
) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sheet sync transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	timestamp := time.Now().UTC().Format(time.RFC3339)

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE jobs
			SET sheet_synced_at = ?
			WHERE id = ?
		`, timestamp, id); err != nil {
			return fmt.Errorf("mark job %d as synced: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sheet sync transaction: %w", err)
	}

	return nil
}
