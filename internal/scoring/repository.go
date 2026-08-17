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
	ID                   int64
	JobID                int64
	OverallScore         float64
	SkillsScore          *float64
	CandidateSkillsScore *float64
	ExperienceScore      *float64
	LocationScore        *float64
	SeniorityScore       *float64
	CompensationScore    *float64
	RoleScore            *float64
	DomainScore          *float64
	CandidateDomainScore *float64
	CompanyScore         *float64
	Recommendation       Recommendation
	Reasoning            string
	CreatedAt            time.Time
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
		ON CONFLICT(job_id)
		DO UPDATE SET
			overall_score = excluded.overall_score,
			skills_score = excluded.skills_score,
			experience_score = excluded.experience_score,
			location_score = excluded.location_score,
			seniority_score = excluded.seniority_score,
			compensation_score = excluded.compensation_score,
			role_score = excluded.role_score,
			domain_score = excluded.domain_score,
			company_score = excluded.company_score,
			recommendation = excluded.recommendation,
			reasoning = excluded.reasoning,
			created_at = CURRENT_TIMESTAMP
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

func (r *Repository) List(
	ctx context.Context,
	limit int,
) ([]StoredScore, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
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
		ORDER BY overall_score DESC, id ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list job scores: %w", err)
	}
	defer rows.Close()

	var scores []StoredScore

	for rows.Next() {
		var (
			score          StoredScore
			recommendation string
			createdAt      string
		)

		if err := rows.Scan(
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
		); err != nil {
			return nil, fmt.Errorf("scan job score: %w", err)
		}

		score.Recommendation = Recommendation(recommendation)

		t, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse score created_at: %w", err)
		}

		score.CreatedAt = t
		scores = append(scores, score)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job scores: %w", err)
	}

	return scores, nil
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

func (r *Repository) ListLatest(
	ctx context.Context,
	limit int,
) ([]StoredScore, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			job_id,
			overall_score,
			skills_score,
			candidate_skills_score,
			experience_score,
			location_score,
			seniority_score,
			compensation_score,
			role_score,
			domain_score,
			candidate_domain_score,
			company_score,
			recommendation,
			COALESCE(reasoning, ''),
			created_at
		FROM job_scores
		ORDER BY overall_score DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list latest job scores: %w", err)
	}
	defer rows.Close()

	var scores []StoredScore

	for rows.Next() {
		var (
			score          StoredScore
			recommendation string
			createdAt      string
		)

		if err := rows.Scan(
			&score.ID,
			&score.JobID,
			&score.OverallScore,
			&score.SkillsScore,
			&score.CandidateSkillsScore,
			&score.ExperienceScore,
			&score.LocationScore,
			&score.SeniorityScore,
			&score.CompensationScore,
			&score.RoleScore,
			&score.DomainScore,
			&score.CandidateDomainScore,
			&score.CompanyScore,
			&recommendation,
			&score.Reasoning,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan job score: %w", err)
		}

		score.Recommendation = Recommendation(recommendation)

		t, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf(
				"parse score created_at: %w",
				err,
			)
		}

		score.CreatedAt = t
		scores = append(scores, score)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job scores: %w", err)
	}

	return scores, nil
}
