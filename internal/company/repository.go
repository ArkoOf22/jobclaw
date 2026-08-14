package company

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jobclaw/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetOrCreate(
	ctx context.Context,
	name string,
) (*Company, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("company name is required")
	}

	normalized := NormalizeName(name)
	if normalized == "" {
		return nil, fmt.Errorf("company name normalizes to empty value")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO companies (
			name,
			normalized_name
		)
		VALUES (?, ?)
		ON CONFLICT(normalized_name)
		DO NOTHING
	`, name, normalized)
	if err != nil {
		return nil, fmt.Errorf("upsert company: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			normalized_name,
			classification,
			classification_reason,
			classified_at,
			created_at
		FROM companies
		WHERE normalized_name = ?
	`, normalized)

	var (
		c                    Company
		classifiedAt         sql.NullString
		classificationReason sql.NullString
		createdAt            string
		classification       string
	)

	if err := row.Scan(
		&c.ID,
		&c.Name,
		&c.NormalizedName,
		&classification,
		&classificationReason,
		&classifiedAt,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	c.Classification = Classification(classification)

	if classificationReason.Valid {
		c.ClassificationReason = classificationReason.String
	}

	if classifiedAt.Valid && classifiedAt.String != "" {
		t, err := parseTime(classifiedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse classified_at: %w", err)
		}

		c.ClassifiedAt = &t
	}

	if createdAt != "" {
		t, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}

		c.CreatedAt = t
	}

	return &c, nil
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

func (r *Repository) GetByID(
	ctx context.Context,
	id int64,
) (*Company, error) {
	if id <= 0 {
		return nil, fmt.Errorf("company ID must be positive")
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			name,
			normalized_name,
			classification,
			classification_reason,
			classified_at,
			created_at
		FROM companies
		WHERE id = ?
	`, id)

	var (
		c                    Company
		classification       string
		classificationReason sql.NullString
		classifiedAt         sql.NullString
		createdAt            string
	)

	if err := row.Scan(
		&c.ID,
		&c.Name,
		&c.NormalizedName,
		&classification,
		&classificationReason,
		&classifiedAt,
		&createdAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("company %d not found", id)
		}

		return nil, fmt.Errorf("get company by ID: %w", err)
	}

	c.Classification = Classification(classification)

	if classificationReason.Valid {
		c.ClassificationReason = classificationReason.String
	}

	if classifiedAt.Valid && classifiedAt.String != "" {
		t, err := parseTime(classifiedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse classified_at: %w", err)
		}

		c.ClassifiedAt = &t
	}

	if createdAt != "" {
		t, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}

		c.CreatedAt = t
	}

	return &c, nil
}

func (r *Repository) UpdateClassification(
	ctx context.Context,
	id int64,
	classification Classification,
	reason string,
	classifiedAt time.Time,
) error {
	if id <= 0 {
		return fmt.Errorf("company ID must be positive")
	}

	if classification == "" {
		return fmt.Errorf("classification is required")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE companies
		SET
			classification = ?,
			classification_reason = ?,
			classified_at = ?
		WHERE id = ?
	`,
		classification,
		reason,
		classifiedAt.UTC().Format(time.RFC3339),
		id,
	)
	if err != nil {
		return fmt.Errorf("update company classification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check company classification update: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("company %d not found", id)
	}

	return nil
}
