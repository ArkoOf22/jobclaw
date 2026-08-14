package job

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobclaw/internal/company"
	"jobclaw/internal/database"
)

type SQLiteRepository struct {
	db *database.DB
}

func NewSQLiteRepository(db *database.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Upsert(ctx context.Context, j Job) error {
	if j.Source == "" {
		return fmt.Errorf("job source is required")
	}
	if j.ExternalID == "" {
		return fmt.Errorf("job external ID is required")
	}
	if j.Company == "" {
		return fmt.Errorf("job company is required")
	}
	if j.Title == "" {
		return fmt.Errorf("job title is required")
	}
	if j.URL == "" {
		return fmt.Errorf("job URL is required")
	}

	sourceRepo := NewSQLiteSourceRepository(r.db)
	if err := sourceRepo.EnsureSource(ctx, j.Source, ""); err != nil {
		return err
	}

	companyRepo := company.NewRepository(r.db)
	companyRecord, err := companyRepo.GetOrCreate(ctx, j.Company)
	if err != nil {
		return fmt.Errorf("resolve company: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO jobs (
			source_id,
			external_id,
			company,
			company_id,
			title,
			description,
			location,
			remote_type,
			employment_type,
			salary_min,
			salary_max,
			currency,
			url,
			posted_at
		)
		SELECT
			id,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		FROM job_sources
		WHERE name = ?
		ON CONFLICT(source_id, external_id)
		DO UPDATE SET
			company = excluded.company,
			company_id = excluded.company_id,
			title = excluded.title,
			description = excluded.description,
			location = excluded.location,
			remote_type = excluded.remote_type,
			employment_type = excluded.employment_type,
			salary_min = excluded.salary_min,
			salary_max = excluded.salary_max,
			currency = excluded.currency,
			url = excluded.url,
			posted_at = excluded.posted_at
	`,
		j.ExternalID,
		j.Company,
		companyRecord.ID,
		j.Title,
		j.Description,
		j.Location,
		j.RemoteType,
		j.EmploymentType,
		j.SalaryMin,
		j.SalaryMax,
		j.Currency,
		j.URL,
		formatTime(j.PostedAt),
		j.Source,
	)

	if err != nil {
		return fmt.Errorf("upsert job: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) GetBySourceExternalID(
	ctx context.Context,
	source string,
	externalID string,
) (*Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			j.external_id,
			js.name,
			j.company,
			j.title,
			j.description,
			j.location,
			j.remote_type,
			j.employment_type,
			j.salary_min,
			j.salary_max,
			j.currency,
			j.url,
			j.posted_at,
			j.discovered_at
		FROM jobs j
		JOIN job_sources js ON js.id = j.source_id
		WHERE js.name = ?
		  AND j.external_id = ?
	`,
		source,
		externalID,
	)

	var (
		j            Job
		postedAt     sql.NullString
		discoveredAt string
	)

	err := row.Scan(
		&j.ExternalID,
		&j.Source,
		&j.Company,
		&j.Title,
		&j.Description,
		&j.Location,
		&j.RemoteType,
		&j.EmploymentType,
		&j.SalaryMin,
		&j.SalaryMax,
		&j.Currency,
		&j.URL,
		&postedAt,
		&discoveredAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get job: %w", err)
	}

	if postedAt.Valid && postedAt.String != "" {
		t, err := parseTime(postedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse posted_at: %w", err)
		}

		j.PostedAt = &t
	}

	if discoveredAt != "" {
		t, err := parseTime(discoveredAt)
		if err != nil {
			return nil, fmt.Errorf("parse discovered_at: %w", err)
		}

		j.DiscoveredAt = t
	}

	return &j, nil
}

func (r *SQLiteRepository) List(
	ctx context.Context,
	limit int,
) ([]Job, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			j.external_id,
			js.name,
			j.company,
			j.title,
			j.description,
			j.location,
			j.remote_type,
			j.employment_type,
			j.salary_min,
			j.salary_max,
			j.currency,
			j.url,
			j.posted_at,
			j.discovered_at
		FROM jobs j
		JOIN job_sources js ON js.id = j.source_id
		ORDER BY j.discovered_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job

	for rows.Next() {
		var (
			j            Job
			postedAt     sql.NullString
			discoveredAt string
		)

		if err := rows.Scan(
			&j.ExternalID,
			&j.Source,
			&j.Company,
			&j.Title,
			&j.Description,
			&j.Location,
			&j.RemoteType,
			&j.EmploymentType,
			&j.SalaryMin,
			&j.SalaryMax,
			&j.Currency,
			&j.URL,
			&postedAt,
			&discoveredAt,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}

		if postedAt.Valid && postedAt.String != "" {
			t, err := parseTime(postedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse posted_at: %w", err)
			}

			j.PostedAt = &t
		}

		if discoveredAt != "" {
			t, err := parseTime(discoveredAt)
			if err != nil {
				return nil, fmt.Errorf("parse discovered_at: %w", err)
			}

			j.DiscoveredAt = t
		}

		jobs = append(jobs, j)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}

	return jobs, nil
}

func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}

	return t.UTC().Format(time.RFC3339)
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
