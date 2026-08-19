package application

import (
	"context"
	"database/sql"
	"fmt"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

type SQLiteSubmissionTransactionFactory struct {
	db *database.DB
}

func NewSQLiteSubmissionTransactionFactory(
	db *database.DB,
) *SQLiteSubmissionTransactionFactory {
	return &SQLiteSubmissionTransactionFactory{
		db: db,
	}
}

func (f *SQLiteSubmissionTransactionFactory) BeginSubmissionTransaction(
	ctx context.Context,
) (SubmissionTransaction, error) {
	if f == nil || f.db == nil {
		return nil, fmt.Errorf("database is not configured")
	}

	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	return &sqliteSubmissionTransaction{
		tx: tx,
	}, nil
}

type sqliteSubmissionTransaction struct {
	tx *sql.Tx
}

func (t *sqliteSubmissionTransaction) UpdateApplicationStatus(
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

	result, err := t.tx.ExecContext(ctx, `
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

func (t *sqliteSubmissionTransaction) UpdateJobStatus(
	ctx context.Context,
	id int64,
	status job.Status,
) error {
	if id <= 0 {
		return fmt.Errorf("job ID must be positive")
	}

	if status == "" {
		return fmt.Errorf("job status is required")
	}

	result, err := t.tx.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?
		WHERE id = ?
	`,
		string(status),
		id,
	)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check job status update: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job %d not found", id)
	}

	return nil
}

func (t *sqliteSubmissionTransaction) CreateEvent(
	ctx context.Context,
	event Event,
) error {
	if event.ApplicationID <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if event.Type == "" {
		return fmt.Errorf("event type is required")
	}

	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO application_events (
			application_id,
			event_type,
			metadata
		)
		VALUES (?, ?, ?)
	`,
		event.ApplicationID,
		event.Type,
		event.Metadata,
	)
	if err != nil {
		return fmt.Errorf("create application event: %w", err)
	}

	return nil
}

func (t *sqliteSubmissionTransaction) Commit() error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("transaction is not configured")
	}

	return t.tx.Commit()
}

func (t *sqliteSubmissionTransaction) Rollback() error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("transaction is not configured")
	}

	return t.tx.Rollback()
}
