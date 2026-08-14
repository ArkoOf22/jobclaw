package job

import (
	"context"
	"fmt"

	"jobclaw/internal/database"
)

type SQLiteSourceRepository struct {
	db *database.DB
}

func NewSQLiteSourceRepository(db *database.DB) *SQLiteSourceRepository {
	return &SQLiteSourceRepository{db: db}
}

func (r *SQLiteSourceRepository) EnsureSource(
	ctx context.Context,
	name string,
	baseURL string,
) error {
	if name == "" {
		return fmt.Errorf("source name is required")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO job_sources (name, base_url)
		VALUES (?, ?)
		ON CONFLICT(name)
		DO UPDATE SET base_url = excluded.base_url
	`, name, baseURL)

	if err != nil {
		return fmt.Errorf("ensure source: %w", err)
	}

	return nil
}
