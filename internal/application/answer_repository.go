package application

import (
	"context"
	"database/sql"
	"fmt"

	"jobclaw/internal/database"
)

type AnswerRepository interface {
	Create(ctx context.Context, answer CandidateAnswer) error

	GetByID(
		ctx context.Context,
		id int64,
	) (*CandidateAnswer, error)

	GetByFieldKey(
		ctx context.Context,
		fieldKey string,
	) (*CandidateAnswer, error)

	List(
		ctx context.Context,
	) ([]CandidateAnswer, error)

	Update(
		ctx context.Context,
		answer CandidateAnswer,
	) error
}

type SQLiteAnswerRepository struct {
	db *database.DB
}

func NewSQLiteAnswerRepository(
	db *database.DB,
) *SQLiteAnswerRepository {
	return &SQLiteAnswerRepository{
		db: db,
	}
}

func (r *SQLiteAnswerRepository) Create(
	ctx context.Context,
	answer CandidateAnswer,
) error {
	if answer.FieldKey == "" {
		return fmt.Errorf("field key is required")
	}

	if answer.Answer == "" {
		return fmt.Errorf("answer is required")
	}

	if answer.ValueType == "" {
		answer.ValueType = AnswerValueText
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO candidate_answers (
			field_key,
			question,
			answer,
			value_type,
			verified,
			notes
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		answer.FieldKey,
		answer.Question,
		answer.Answer,
		answer.ValueType,
		answer.Verified,
		answer.Notes,
	)

	if err != nil {
		return fmt.Errorf("create candidate answer: %w", err)
	}

	return nil
}

func (r *SQLiteAnswerRepository) GetByID(
	ctx context.Context,
	id int64,
) (*CandidateAnswer, error) {
	if id <= 0 {
		return nil, fmt.Errorf("answer ID must be positive")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			field_key,
			COALESCE(question, ''),
			answer,
			value_type,
			verified,
			COALESCE(notes, ''),
			created_at,
			updated_at
		FROM candidate_answers
		WHERE id = ?
	`, id)

	var (
		answer    CandidateAnswer
		valueType string
		verified  bool
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&answer.ID,
		&answer.FieldKey,
		&answer.Question,
		&answer.Answer,
		&valueType,
		&verified,
		&answer.Notes,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get candidate answer: %w", err)
	}

	answer.ValueType = AnswerValueType(valueType)
	answer.Verified = verified

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse candidate answer created_at: %w",
			err,
		)
	}

	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse candidate answer updated_at: %w",
			err,
		)
	}

	answer.CreatedAt = created
	answer.UpdatedAt = updated

	return &answer, nil
}

func (r *SQLiteAnswerRepository) GetByFieldKey(
	ctx context.Context,
	fieldKey string,
) (*CandidateAnswer, error) {
	if fieldKey == "" {
		return nil, fmt.Errorf("field key is required")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			field_key,
			COALESCE(question, ''),
			answer,
			value_type,
			verified,
			COALESCE(notes, ''),
			created_at,
			updated_at
		FROM candidate_answers
		WHERE field_key = ?
		LIMIT 1
	`, fieldKey)

	var (
		answer    CandidateAnswer
		valueType string
		verified  bool
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&answer.ID,
		&answer.FieldKey,
		&answer.Question,
		&answer.Answer,
		&valueType,
		&verified,
		&answer.Notes,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf(
			"get candidate answer by field key: %w",
			err,
		)
	}

	answer.ValueType = AnswerValueType(valueType)
	answer.Verified = verified

	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse candidate answer created_at: %w",
			err,
		)
	}

	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf(
			"parse candidate answer updated_at: %w",
			err,
		)
	}

	answer.CreatedAt = created
	answer.UpdatedAt = updated

	return &answer, nil
}

func (r *SQLiteAnswerRepository) List(
	ctx context.Context,
) ([]CandidateAnswer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			field_key,
			COALESCE(question, ''),
			answer,
			value_type,
			verified,
			COALESCE(notes, ''),
			created_at,
			updated_at
		FROM candidate_answers
		ORDER BY field_key ASC
	`)

	if err != nil {
		return nil, fmt.Errorf(
			"list candidate answers: %w",
			err,
		)
	}

	defer rows.Close()

	var answers []CandidateAnswer

	for rows.Next() {
		var (
			answer    CandidateAnswer
			valueType string
			verified  bool
			createdAt string
			updatedAt string
		)

		if err := rows.Scan(
			&answer.ID,
			&answer.FieldKey,
			&answer.Question,
			&answer.Answer,
			&valueType,
			&verified,
			&answer.Notes,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf(
				"scan candidate answer: %w",
				err,
			)
		}

		answer.ValueType = AnswerValueType(valueType)
		answer.Verified = verified

		created, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse candidate answer created_at: %w",
				err,
			)
		}

		updated, err := parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse candidate answer updated_at: %w",
				err,
			)
		}

		answer.CreatedAt = created
		answer.UpdatedAt = updated

		answers = append(answers, answer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate candidate answers: %w",
			err,
		)
	}

	if answers == nil {
		answers = []CandidateAnswer{}
	}

	return answers, nil
}

func (r *SQLiteAnswerRepository) Update(
	ctx context.Context,
	answer CandidateAnswer,
) error {
	if answer.ID <= 0 {
		return fmt.Errorf("answer ID must be positive")
	}

	if answer.FieldKey == "" {
		return fmt.Errorf("field key is required")
	}

	if answer.Answer == "" {
		return fmt.Errorf("answer is required")
	}

	if answer.ValueType == "" {
		answer.ValueType = AnswerValueText
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE candidate_answers
		SET
			field_key = ?,
			question = ?,
			answer = ?,
			value_type = ?,
			verified = ?,
			notes = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		answer.FieldKey,
		answer.Question,
		answer.Answer,
		answer.ValueType,
		answer.Verified,
		answer.Notes,
		answer.ID,
	)

	if err != nil {
		return fmt.Errorf("update candidate answer: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"check candidate answer update: %w",
			err,
		)
	}

	if rows == 0 {
		return fmt.Errorf(
			"candidate answer %d not found",
			answer.ID,
		)
	}

	return nil
}
