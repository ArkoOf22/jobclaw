package scoring

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobclaw/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

type StoredScore struct {
	ID                int64
	JobID             int64
	OverallScore      float64
	SkillsScore       *float64
	ExperienceScore   *float64
	LocationScore     *float64
	SeniorityScore    *float64
	CompensationScore *float64
	RoleScore         *float64
	DomainScore       *float64
	CompanyScore      *float64
	Recommendation    Recommendation
	Reasoning         string
	CreatedAt         time.Time
}

func (r *Repository) Save(
	ctx context.Context,
	jobID int64,
	result Result,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO job_scores (
			job_id,
			overall_score,
			skills_score,
			experience_score,
			location_score,
			seniority_score,
			compensation_score,
			role_score,
			domain_score,
			company_score,
			recommendation,
			reasoning
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		jobID,
		result.OverallScore,
		result.SkillsScore,
		result.ExperienceScore,
		result.LocationScore,
		nil,
		result.CompensationScore,
		result.RoleScore,
		result.DomainScore,
		result.CompanyScore,
		result.Recommendation,
		result.Reasoning,
	)
	if err != nil {
		return fmt.Errorf("save job score: %w", err)
	}

	return nil
}

func (r *Repository) GetLatest(
	ctx context.Context,
	jobID int64,
) (*StoredScore, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			job_id,
			overall_score,
			skills_score,
			experience_score,
			location_score,
			seniority_score,
			compensation_score,
			role_score,
			domain_score,
			company_score,
			recommendation,
			COALESCE(reasoning, ''),
			created_at
		FROM job_scores
		WHERE job_id = ?
		ORDER BY id DESC
		LIMIT 1
	`, jobID)

	var (
		score          StoredScore
		recommendation string
		createdAt      string
	)

	err := row.Scan(
		&score.ID,
		&score.JobID,
		&score.OverallScore,
		&score.SkillsScore,
		&score.ExperienceScore,
		&score.LocationScore,
		&score.SeniorityScore,
		&score.CompensationScore,
		&score.RoleScore,
		&score.DomainScore,
		&score.CompanyScore,
		&recommendation,
		&score.Reasoning,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("get latest job score: %w", err)
	}

	score.Recommendation = Recommendation(recommendation)

	t, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse score created_at: %w", err)
	}

	score.CreatedAt = t

	return &score, nil
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
