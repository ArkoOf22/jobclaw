package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobclaw/internal/database"
)

type SQLiteRepository struct {
	db *database.DB
}

func NewSQLiteRepository(db *database.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Create(
	ctx context.Context,
	app Application,
) error {
	if app.JobID <= 0 {
		return fmt.Errorf("job ID must be positive")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO applications (
			job_id,
			status,
			tailored_resume_path,
			cover_letter_path,
			referral_message_path,
			application_answers_path
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		app.JobID,
		app.Status,
		app.TailoredResumePath,
		app.CoverLetterPath,
		app.ReferralMessagePath,
		app.ApplicationAnswersPath,
	)
	if err != nil {
		return fmt.Errorf("create application: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) GetByID(
	ctx context.Context,
	id int64,
) (*Application, error) {
	if id <= 0 {
		return nil, fmt.Errorf("application ID must be positive")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			job_id,
			status,
			COALESCE(tailored_resume_path, ''),
			COALESCE(cover_letter_path, ''),
			COALESCE(referral_message_path, ''),
			COALESCE(application_answers_path, ''),
			created_at,
			updated_at
		FROM applications
		WHERE id = ?
		LIMIT 1
	`, id)

	var (
		app       Application
		status    string
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&app.ID,
		&app.JobID,
		&status,
		&app.TailoredResumePath,
		&app.CoverLetterPath,
		&app.ReferralMessagePath,
		&app.ApplicationAnswersPath,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get application by ID: %w", err)
	}

	app.Status = Status(status)

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse application created_at: %w",
			err,
		)
	}

	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse application updated_at: %w",
			err,
		)
	}

	app.CreatedAt = created
	app.UpdatedAt = updated

	return &app, nil
}

func (r *SQLiteRepository) GetByJobID(
	ctx context.Context,
	jobID int64,
) (*Application, error) {
	if jobID <= 0 {
		return nil, fmt.Errorf("job ID must be positive")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			job_id,
			status,
			COALESCE(tailored_resume_path, ''),
			COALESCE(cover_letter_path, ''),
			COALESCE(referral_message_path, ''),
			COALESCE(application_answers_path, ''),
			created_at,
			updated_at
		FROM applications
		WHERE job_id = ?
		LIMIT 1
	`, jobID)

	var (
		app       Application
		status    string
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&app.ID,
		&app.JobID,
		&status,
		&app.TailoredResumePath,
		&app.CoverLetterPath,
		&app.ReferralMessagePath,
		&app.ApplicationAnswersPath,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get application by job ID: %w", err)
	}

	app.Status = Status(status)

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse application created_at: %w", err)
	}

	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse application updated_at: %w", err)
	}

	app.CreatedAt = created
	app.UpdatedAt = updated

	return &app, nil
}

func (r *SQLiteRepository) UpdateTailoredResumePath(
	ctx context.Context,
	id int64,
	path string,
) error {
	if id <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if path == "" {
		return fmt.Errorf("tailored resume path is required")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE applications
		SET
			tailored_resume_path = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		path,
		id,
	)
	if err != nil {
		return fmt.Errorf("update tailored resume path: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check tailored resume path update: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("application %d not found", id)
	}

	return nil
}

func (r *SQLiteRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status Status,
) error {
	if id <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if status == "" {
		return fmt.Errorf("application status is required")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE applications
		SET
			status = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		status,
		id,
	)
	if err != nil {
		return fmt.Errorf("update application status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check application status update: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("application %d not found", id)
	}

	return nil
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

// List returns applications newest first.
//
// Deliberately not part of the Repository interface: only the status report needs
// it, and widening the interface would force every implementation and test fake
// to grow a method they do not use.
func (r *SQLiteRepository) List(
	ctx context.Context,
	limit int,
) ([]Application, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			job_id,
			status,
			COALESCE(tailored_resume_path, ''),
			COALESCE(cover_letter_path, ''),
			COALESCE(referral_message_path, ''),
			COALESCE(application_answers_path, ''),
			created_at,
			updated_at
		FROM applications
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()

	applications := make([]Application, 0)

	for rows.Next() {
		var (
			app       Application
			status    string
			createdAt string
			updatedAt string
		)

		if err := rows.Scan(
			&app.ID,
			&app.JobID,
			&status,
			&app.TailoredResumePath,
			&app.CoverLetterPath,
			&app.ReferralMessagePath,
			&app.ApplicationAnswersPath,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}

		app.Status = Status(status)

		created, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse application created_at: %w",
				err,
			)
		}

		updated, err := parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse application updated_at: %w",
				err,
			)
		}

		app.CreatedAt = created
		app.UpdatedAt = updated

		applications = append(applications, app)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applications: %w", err)
	}

	return applications, nil
}
