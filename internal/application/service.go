package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"jobclaw/internal/job"
)

type Service struct {
	jobs         *job.SQLiteRepository
	applications *SQLiteRepository
	events       EventRepository
	resume       ResumeGenerator
}

func NewService(
	jobs *job.SQLiteRepository,
	applications *SQLiteRepository,
	events EventRepository,
	resume ResumeGenerator,
) *Service {
	return &Service{
		jobs:         jobs,
		applications: applications,
		events:       events,
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

func (s *Service) recordEvent(
	ctx context.Context,
	applicationID int64,
	eventType EventType,
	metadata any,
) error {
	if s.events == nil {
		return nil
	}

	metadataJSON := ""

	if metadata != nil {
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal event metadata: %w", err)
		}

		metadataJSON = string(encoded)
	}

	return s.events.Create(ctx, Event{
		ApplicationID: applicationID,
		Type:          eventType,
		Metadata:      metadataJSON,
	})
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

	root, err := ensureApplicationWorkspace(jobID)
	if err != nil {
		return nil, err
	}

	if s.resume == nil {
		return nil, fmt.Errorf("resume generator is not configured")
	}

	resumePath := filepath.Join(
		root,
		"resume",
		"tailored_resume.txt",
	)

	// Create the application before generation so every lifecycle event
	// has a valid application ID.
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

	// Event persistence is observational. Failure to write an event must
	// not make the application workflow fail.
	_ = s.recordEvent(
		ctx,
		created.ID,
		EventApplicationCreated,
		map[string]any{
			"job_id": created.JobID,
		},
	)

	_ = s.recordEvent(
		ctx,
		created.ID,
		EventResumeGenerationStarted,
		map[string]any{
			"job_id": jobID,
		},
	)

	if err := s.resume.GenerateTailoredResume(
		ctx,
		jobID,
		resumePath,
	); err != nil {
		_ = s.recordEvent(
			ctx,
			created.ID,
			EventResumeGenerationFailed,
			map[string]any{
				"error": err.Error(),
			},
		)

		return nil, fmt.Errorf("generate tailored resume: %w", err)
	}

	if err := s.applications.UpdateTailoredResumePath(
		ctx,
		created.ID,
		resumePath,
	); err != nil {
		_ = s.recordEvent(
			ctx,
			created.ID,
			EventResumeGenerationFailed,
			map[string]any{
				"error": err.Error(),
			},
		)

		return nil, fmt.Errorf("update tailored resume path: %w", err)
	}

	_ = s.recordEvent(
		ctx,
		created.ID,
		EventResumeGenerationSucceeded,
		map[string]any{
			"resume_path": resumePath,
		},
	)

	created, err = s.applications.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("load updated application: %w", err)
	}

	if created == nil {
		return nil, fmt.Errorf("application was updated but could not be loaded")
	}

	return created, nil
}
