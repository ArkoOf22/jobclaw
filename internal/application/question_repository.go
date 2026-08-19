package application

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"jobclaw/internal/database"
)

type QuestionRepository interface {
	Create(ctx context.Context, question ApplicationQuestion) error
	GetByID(ctx context.Context, id int64) (*ApplicationQuestion, error)
	FindByApplicationAndQuestion(
		ctx context.Context,
		applicationID int64,
		question string,
	) (*ApplicationQuestion, error)
	ListByApplicationID(
		ctx context.Context,
		applicationID int64,
	) ([]ApplicationQuestion, error)
	UpdateAnswer(
		ctx context.Context,
		id int64,
		answer string,
		source AnswerSource,
		status QuestionStatus,
	) error
}

type SQLiteQuestionRepository struct {
	db *database.DB
}

func NewSQLiteQuestionRepository(
	db *database.DB,
) *SQLiteQuestionRepository {
	return &SQLiteQuestionRepository{db: db}
}

func (r *SQLiteQuestionRepository) Create(
	ctx context.Context,
	question ApplicationQuestion,
) error {
	if question.ApplicationID <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if question.Question == "" {
		return fmt.Errorf("question is required")
	}

	if question.Status == "" {
		question.Status = QuestionNeedsReview
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO application_questions (
			application_id,
			question,
			field_key,
			answer,
			answer_source,
			status,
			metadata
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(application_id, question) DO NOTHING
	`,
		question.ApplicationID,
		question.Question,
		question.FieldKey,
		question.Answer,
		question.AnswerSource,
		question.Status,
		question.Metadata,
	)

	if err != nil {
		return fmt.Errorf("create application question: %w", err)
	}

	return nil
}

func (r *SQLiteQuestionRepository) GetByID(
	ctx context.Context,
	id int64,
) (*ApplicationQuestion, error) {
	if id <= 0 {
		return nil, fmt.Errorf("question ID must be positive")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			application_id,
			question,
			COALESCE(field_key, ''),
			COALESCE(answer, ''),
			COALESCE(answer_source, ''),
			status,
			COALESCE(metadata, ''),
			created_at,
			updated_at
		FROM application_questions
		WHERE id = ?
	`, id)

	var (
		q         ApplicationQuestion
		source    string
		status    string
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&q.ID,
		&q.ApplicationID,
		&q.Question,
		&q.FieldKey,
		&q.Answer,
		&source,
		&status,
		&q.Metadata,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get application question: %w", err)
	}

	q.AnswerSource = AnswerSource(source)
	q.Status = QuestionStatus(status)

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse question created_at: %w", err)
	}

	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse question updated_at: %w", err)
	}

	q.CreatedAt = created
	q.UpdatedAt = updated

	return &q, nil
}

func (r *SQLiteQuestionRepository) FindByApplicationAndQuestion(
	ctx context.Context,
	applicationID int64,
	question string,
) (*ApplicationQuestion, error) {
	if applicationID <= 0 {
		return nil, fmt.Errorf("application ID must be positive")
	}

	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			application_id,
			question,
			COALESCE(field_key, ''),
			COALESCE(answer, ''),
			COALESCE(answer_source, ''),
			status,
			COALESCE(metadata, ''),
			created_at,
			updated_at
		FROM application_questions
		WHERE application_id = ?
		  AND question = ?
		ORDER BY id ASC
		LIMIT 1
	`,
		applicationID,
		question,
	)

	var (
		q         ApplicationQuestion
		source    string
		status    string
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&q.ID,
		&q.ApplicationID,
		&q.Question,
		&q.FieldKey,
		&q.Answer,
		&source,
		&status,
		&q.Metadata,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf(
			"find application question: %w",
			err,
		)
	}

	q.AnswerSource = AnswerSource(source)
	q.Status = QuestionStatus(status)

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse question created_at: %w",
			err,
		)
	}

	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse question updated_at: %w",
			err,
		)
	}

	q.CreatedAt = created
	q.UpdatedAt = updated

	return &q, nil
}

func (r *SQLiteQuestionRepository) ListByApplicationID(
	ctx context.Context,
	applicationID int64,
) ([]ApplicationQuestion, error) {
	if applicationID <= 0 {
		return nil, fmt.Errorf("application ID must be positive")
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			application_id,
			question,
			COALESCE(field_key, ''),
			COALESCE(answer, ''),
			COALESCE(answer_source, ''),
			status,
			COALESCE(metadata, ''),
			created_at,
			updated_at
		FROM application_questions
		WHERE application_id = ?
		ORDER BY id ASC
	`, applicationID)

	if err != nil {
		return nil, fmt.Errorf(
			"list application questions: %w",
			err,
		)
	}

	defer rows.Close()

	var questions []ApplicationQuestion

	for rows.Next() {
		var (
			q         ApplicationQuestion
			source    string
			status    string
			createdAt string
			updatedAt string
		)

		if err := rows.Scan(
			&q.ID,
			&q.ApplicationID,
			&q.Question,
			&q.FieldKey,
			&q.Answer,
			&source,
			&status,
			&q.Metadata,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf(
				"scan application question: %w",
				err,
			)
		}

		q.AnswerSource = AnswerSource(source)
		q.Status = QuestionStatus(status)

		created, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse question created_at: %w",
				err,
			)
		}

		updated, err := parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse question updated_at: %w",
				err,
			)
		}

		q.CreatedAt = created
		q.UpdatedAt = updated

		questions = append(questions, q)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate application questions: %w",
			err,
		)
	}

	if questions == nil {
		questions = []ApplicationQuestion{}
	}

	return questions, nil
}

func (r *SQLiteQuestionRepository) UpdateAnswer(
	ctx context.Context,
	id int64,
	answer string,
	source AnswerSource,
	status QuestionStatus,
) error {
	if id <= 0 {
		return fmt.Errorf("question ID must be positive")
	}

	if status == "" {
		return fmt.Errorf("question status is required")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE application_questions
		SET
			answer = ?,
			answer_source = ?,
			status = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		answer,
		source,
		status,
		id,
	)

	if err != nil {
		return fmt.Errorf(
			"update application question answer: %w",
			err,
		)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"check application question update: %w",
			err,
		)
	}

	if rows == 0 {
		return fmt.Errorf("application question %d not found", id)
	}

	return nil
}
