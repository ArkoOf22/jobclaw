package scoring

import (
	"context"
	"fmt"

	"jobclaw/internal/company"
	"jobclaw/internal/job"
)

// CompanyClassifier derives and persists a company's classification from
// stored evidence. *company.Service satisfies this.
//
// Scoring reads Company.Classification, and when RequireProductCompany is set a
// non-PRODUCT classification forces a SKIP recommendation regardless of score.
// Nothing else in the discover -> score path ever classified a company, so
// every company stayed UNKNOWN and every job was skipped. Scoring therefore
// classifies on demand rather than trusting that some earlier step did it.
type CompanyClassifier interface {
	Classify(ctx context.Context, companyID int64) (*company.Company, error)
}

type Service struct {
	jobs       *job.SQLiteRepository
	companies  *company.Repository
	repository *Repository
	scorer     *Scorer

	classifier CompanyClassifier
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

// WithCompanyClassifier enables on-demand classification of companies that have
// not been classified yet. Optional, so existing callers keep working; when
// unset, scoring uses whatever classification is already stored.
func (s *Service) WithCompanyClassifier(
	classifier CompanyClassifier,
) *Service {
	s.classifier = classifier

	return s
}

// targetStatusFor maps a recommendation to the job status it should advance to.
//
// APPLY is a stronger recommendation than SHORTLIST and must not leave the job
// in a weaker status. Previously only SHORTLIST advanced the job while APPLY
// fell through to SCORED, so the best matches ranked below merely-good ones.
func targetStatusFor(recommendation Recommendation) job.Status {
	switch recommendation {
	case RecommendationApply, RecommendationShortlist:
		return job.StatusShortlisted
	default:
		return job.StatusScored
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

	// An unclassified company scores as UNKNOWN, which under
	// RequireProductCompany silently forces SKIP. Classify now rather than
	// scoring against a value we know is missing. Errors propagate: a wrong
	// classification changes the recommendation, so it must not fail quietly.
	if s.classifier != nil &&
		c.Classification == company.ClassificationUnknown {
		classified, err := s.classifier.Classify(ctx, companyID)
		if err != nil {
			return nil, fmt.Errorf(
				"classify company %d: %w",
				companyID,
				err,
			)
		}

		if classified != nil {
			c = classified
		}
	}

	result := s.scorer.Score(*j, *c)

	if err := s.repository.Save(ctx, jobID, result); err != nil {
		return nil, fmt.Errorf("persist score: %w", err)
	}

	targetStatus := targetStatusFor(result.Recommendation)

	if j.Status != targetStatus &&
		job.CanTransition(j.Status, targetStatus) {
		if err := s.jobs.UpdateStatus(ctx, jobID, targetStatus); err != nil {
			return nil, fmt.Errorf("update job status: %w", err)
		}
	}

	return &result, nil
}
