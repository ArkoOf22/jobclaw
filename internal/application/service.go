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

	return created, nil
}

// EnsureTailoredResume generates the tailored resume for an existing
// application, and is safe to call repeatedly.
//
// This is deliberately separate from CreateForApprovedJob. The two were fused,
// which produced an unrecoverable state: the application row was created first,
// so when resume generation failed the caller got an error while the row
// persisted. The next attempt then matched the "application already exists"
// branch and returned success without ever generating the resume, leaving the
// application permanently resume-less while reporting success.
//
// Splitting them means workspace creation no longer depends on the LLM being
// reachable, and a failed resume can simply be retried.
func (s *Service) EnsureTailoredResume(
	ctx context.Context,
	jobID int64,
) (string, error) {
	if jobID <= 0 {
		return "", fmt.Errorf("job ID must be positive")
	}

	if s.resume == nil {
		return "", fmt.Errorf("resume generator is not configured")
	}

	app, err := s.applications.GetByJobID(ctx, jobID)
	if err != nil {
		return "", fmt.Errorf("load application: %w", err)
	}

	if app == nil {
		return "", fmt.Errorf(
			"no application exists for job %d; create it first",
			jobID,
		)
	}

	root, err := ensureApplicationWorkspace(jobID)
	if err != nil {
		return "", err
	}

	resumePath := filepath.Join(
		root,
		"resume",
		"tailored_resume.txt",
	)

	// Treat a recorded path whose file is missing as not generated, so a
	// deleted or truncated artifact is regenerated rather than trusted.
	if app.TailoredResumePath != "" {
		if info, statErr := os.Stat(
			app.TailoredResumePath,
		); statErr == nil && info.Size() > 0 {
			return app.TailoredResumePath, nil
		}
	}

	_ = s.recordEvent(
		ctx,
		app.ID,
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
			app.ID,
			EventResumeGenerationFailed,
			map[string]any{
				"error": err.Error(),
			},
		)

		return "", fmt.Errorf("generate tailored resume: %w", err)
	}

	if err := s.applications.UpdateTailoredResumePath(
		ctx,
		app.ID,
		resumePath,
	); err != nil {
		_ = s.recordEvent(
			ctx,
			app.ID,
			EventResumeGenerationFailed,
			map[string]any{
				"error": err.Error(),
			},
		)

		return "", fmt.Errorf("update tailored resume path: %w", err)
	}

	_ = s.recordEvent(
		ctx,
		app.ID,
		EventResumeGenerationSucceeded,
		map[string]any{
			"resume_path": resumePath,
		},
	)

	return resumePath, nil
}
