package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"jobclaw/internal/job"
)

type Service struct {
	jobs         *job.SQLiteRepository
	applications *SQLiteRepository
	resume       ResumeGenerator
}

func NewService(
	jobs *job.SQLiteRepository,
	applications *SQLiteRepository,
	resume ResumeGenerator,
) *Service {
	return &Service{
		jobs:         jobs,
		applications: applications,
		resume:       resume,
	}
}

func applicationWorkspace(jobID int64) string {
	return filepath.Join("data", "applications", fmt.Sprintf("%d", jobID))
}

func ensureApplicationWorkspace(jobID int64) (string, error) {
	if jobID <= 0 {
		return "", fmt.Errorf("job ID must be positive")
	}

	root := applicationWorkspace(jobID)

	directories := []string{
		filepath.Join(root, "resume"),
		filepath.Join(root, "cover-letter"),
		filepath.Join(root, "referral"),
		filepath.Join(root, "answers"),
	}

	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return "", fmt.Errorf("create application workspace: %w", err)
		}
	}

	return root, nil
}

func (s *Service) CreateForApprovedJob(
	ctx context.Context,
	jobID int64,
) (*Application, error) {
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

	if j.Status != job.StatusApproved {
		return nil, fmt.Errorf(
			"job %d must be APPROVED before creating an application; current status: %s",
			jobID,
			j.Status,
		)
	}

	existing, err := s.applications.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("check existing application: %w", err)
	}

	if existing != nil {
		if _, err := ensureApplicationWorkspace(jobID); err != nil {
			return nil, err
		}

		return existing, nil
	}

	if _, err := ensureApplicationWorkspace(jobID); err != nil {
		return nil, err
	}

	app := Application{
		JobID:  jobID,
		Status: StatusDraft,
	}

	if err := s.applications.Create(ctx, app); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}

	created, err := s.applications.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("load created application: %w", err)
	}

	if created == nil {
		return nil, fmt.Errorf("application was created but could not be loaded")
	}

	return created, nil
}
