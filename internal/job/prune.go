package job

import (
	"context"
	"fmt"
	"time"
)

// PruneCandidate is a job eligible for removal.
type PruneCandidate struct {
	ID           int64
	Company      string
	Title        string
	Status       Status
	DiscoveredAt time.Time
}

// PruneCriteria selects jobs to retire.
//
// Pruning exists because discovery accumulates: a board sweep stores everything
// that matched at the time, and matching rules improve afterwards. A database
// carrying hundreds of stale rejects makes the real queue hard to see and slows
// every full re-score.
type PruneCriteria struct {
	// Before retires jobs discovered strictly earlier than this instant.
	Before time.Time

	// Statuses limits the sweep. Empty means the safe default set.
	Statuses []Status
}

// prunableStatuses are the only statuses that may ever be removed.
//
// Anything at or past APPROVED may have an application, a generated resume, or a
// real submission attached. Restricting the set here rather than at the call site
// means no caller can widen it by passing different criteria.
//
// SHORTLISTED is deliberately absent. A shortlisted job is an open decision the
// candidate still owes, so bulk-deleting it destroys the queue rather than
// tidying it. The first dry run against real data would have removed three
// shortlisted roles scoring 70 to 77. To retire one, reject it first: that is an
// explicit decision, and REJECTED is prunable.
var prunableStatuses = map[Status]bool{
	StatusDiscovered: true,
	StatusScored:     true,
	StatusRejected:   true,
}

// FindPrunable returns jobs matching the criteria that are safe to delete.
//
// Jobs with an application are excluded regardless of status: an application
// implies generated artifacts and possibly a submission, and deleting the job row
// underneath one would orphan it.
func (r *SQLiteRepository) FindPrunable(
	ctx context.Context,
	criteria PruneCriteria,
) ([]PruneCandidate, error) {
	statuses := criteria.Statuses

	if len(statuses) == 0 {
		statuses = []Status{
			StatusDiscovered,
			StatusScored,
			StatusRejected,
		}
	}

	// Reject anything outside the allowlist rather than silently ignoring it, so
	// a caller asking to prune APPROVED jobs gets an error instead of a
	// misleadingly empty result.
	for _, status := range statuses {
		if !prunableStatuses[status] {
			return nil, fmt.Errorf(
				"status %s may not be pruned",
				status,
			)
		}
	}

	if criteria.Before.IsZero() {
		return nil, fmt.Errorf("prune cutoff is required")
	}

	query := `
		SELECT
			j.id,
			j.company,
			j.title,
			j.status,
			j.discovered_at
		FROM jobs j
		LEFT JOIN applications a ON a.job_id = j.id
		WHERE a.id IS NULL
		  AND j.discovered_at < ?
		  AND j.status IN (`

	args := []any{criteria.Before.UTC().Format(time.RFC3339)}

	for i, status := range statuses {
		if i > 0 {
			query += ","
		}

		query += "?"

		args = append(args, string(status))
	}

	query += `)
		ORDER BY j.id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find prunable jobs: %w", err)
	}
	defer rows.Close()

	candidates := make([]PruneCandidate, 0)

	for rows.Next() {
		var (
			candidate    PruneCandidate
			status       string
			discoveredAt string
		)

		if err := rows.Scan(
			&candidate.ID,
			&candidate.Company,
			&candidate.Title,
			&status,
			&discoveredAt,
		); err != nil {
			return nil, fmt.Errorf("scan prunable job: %w", err)
		}

		candidate.Status = Status(status)

		parsed, err := parseTime(discoveredAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse discovered_at for job %d: %w",
				candidate.ID,
				err,
			)
		}

		candidate.DiscoveredAt = parsed
		candidates = append(candidates, candidate)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prunable jobs: %w", err)
	}

	return candidates, nil
}

// DeleteJobs removes the given jobs and their scores in one transaction.
//
// Takes explicit IDs rather than criteria so the caller deletes exactly what it
// previewed. Re-running a query at delete time could pick up rows that appeared
// in between, which would mean deleting something the operator never saw.
func (r *SQLiteRepository) DeleteJobs(
	ctx context.Context,
	ids []int64,
) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin prune transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	var deleted int64

	for _, id := range ids {
		// Re-check inside the transaction. A job that acquired an application
		// or advanced past a prunable status since the preview must survive.
		var guard int64

		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM jobs j
			LEFT JOIN applications a ON a.job_id = j.id
			WHERE j.id = ?
			  AND a.id IS NULL
			  AND j.status IN (
				'DISCOVERED', 'SCORED', 'REJECTED'
			  )
		`, id).Scan(&guard); err != nil {
			return 0, fmt.Errorf("verify job %d before delete: %w", id, err)
		}

		if guard == 0 {
			continue
		}

		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM job_scores WHERE job_id = ?`,
			id,
		); err != nil {
			return 0, fmt.Errorf("delete scores for job %d: %w", id, err)
		}

		result, err := tx.ExecContext(
			ctx,
			`DELETE FROM jobs WHERE id = ?`,
			id,
		)
		if err != nil {
			return 0, fmt.Errorf("delete job %d: %w", id, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf(
				"count deleted rows for job %d: %w",
				id,
				err,
			)
		}

		deleted += affected
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit prune transaction: %w", err)
	}

	return deleted, nil
}
