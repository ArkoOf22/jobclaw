package scoring

import (
	"context"
	"fmt"

	"jobclaw/internal/company"
	"jobclaw/internal/job"
)

type Service struct {
	jobs       *job.SQLiteRepository
	companies  *company.Repository
	repository *Repository
	scorer     *Scorer
}

func NewService(
	jobs *job.SQLiteRepository,
	companies *company.Repository,
	repository *Repository,
	scorer *Scorer,
) *Service {
	return &Service{
		jobs:       jobs,
		companies:  companies,
		repository: repository,
		scorer:     scorer,
	}
}

func (s *Service) ScoreJob(
	ctx context.Context,
	jobID int64,
) (*Result, error) {
	if jobID <= 0 {
		return nil, fmt.Errorf("job ID must be positive")
	}

	j, err := s.jobs.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("load job: %w", err)
	}

	if j == nil {
		return nil, fmt.Errorf("job %d not found", jobID)
	}

	companyID, err := s.jobs.GetCompanyIDByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("load job company: %w", err)
	}

	c, err := s.companies.GetByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("load company: %w", err)
	}

	if c == nil {
		return nil, fmt.Errorf("company %d not found", companyID)
	}

	result := s.scorer.Score(*j, *c)

	if err := s.repository.Save(ctx, jobID, result); err != nil {
		return nil, fmt.Errorf("persist score: %w", err)
	}

	targetStatus := job.StatusScored

	if result.Recommendation == RecommendationShortlist {
		targetStatus = job.StatusShortlisted
	}

	if j.Status != targetStatus &&
		job.CanTransition(j.Status, targetStatus) {
		if err := s.jobs.UpdateStatus(ctx, jobID, targetStatus); err != nil {
			return nil, fmt.Errorf("update job status: %w", err)
		}
	}

	return &result, nil
}
