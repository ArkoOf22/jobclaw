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
		  AND s.recommendation IN ('SHORTLIST', 'APPLY')
		  AND j.status NOT IN ('REJECTED')
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
