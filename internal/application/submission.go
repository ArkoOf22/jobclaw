package application

import (
	"context"
	"fmt"
	"time"

	"jobclaw/internal/job"
)

type ApplicationSubmitter interface {
	Submit(
		ctx context.Context,
		app Application,
		j job.Job,
	) error
}

type ApplicationSubmissionService struct {
	applications Repository
	jobs         job.Repository
	events       EventRepository
	submitter    ApplicationSubmitter
	transactions SubmissionTransactionFactory
}

func NewApplicationSubmissionService(
	applications Repository,
	jobs job.Repository,
	events EventRepository,
	submitter ApplicationSubmitter,
) *ApplicationSubmissionService {
	return &ApplicationSubmissionService{
		applications: applications,
		jobs:         jobs,
		events:       events,
		submitter:    submitter,
	}
}

func (s *ApplicationSubmissionService) SetTransactionFactory(
	factory SubmissionTransactionFactory,
) {
	s.transactions = factory
}

func (s *ApplicationSubmissionService) Submit(
	ctx context.Context,
	applicationID int64,
) error {
	if applicationID <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if s.applications == nil {
		return fmt.Errorf("application repository is not configured")
	}

	if s.jobs == nil {
		return fmt.Errorf("job repository is not configured")
	}

	if s.submitter == nil {
		return fmt.Errorf("application submitter is not configured")
	}

	app, err := s.applications.GetByID(ctx, applicationID)
	if err != nil {
		return fmt.Errorf("load application: %w", err)
	}

	if app == nil {
		return fmt.Errorf("application %d not found", applicationID)
	}

	if app.Status != StatusReadyToApply {
		return fmt.Errorf(
			"application %d is not READY_TO_APPLY (status=%s)",
			applicationID,
			app.Status,
		)
	}

	j, err := s.jobs.GetByID(ctx, app.JobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}

	if j == nil {
		return fmt.Errorf("job %d not found", app.JobID)
	}

	if j.Status != job.StatusApproved {
		return fmt.Errorf(
			"job %d is not APPROVED (status=%s)",
			j.ID,
			j.Status,
		)
	}

	// A previous attempt may have already crossed the external-system
	// boundary. Never blindly retry an ambiguous submission.
	if app.Status == StatusSubmissionInProgress {
		return fmt.Errorf(
			"application %d has an ambiguous submission attempt; reconciliation is required",
			applicationID,
		)
	}

	// Mark the submission attempt before crossing the external boundary.
	// The attempt marker itself is persisted transactionally.
	attemptID := fmt.Sprintf(
		"submission-%d-%d",
		applicationID,
		time.Now().UnixNano(),
	)

	if s.transactions == nil {
		return fmt.Errorf(
			"submission transaction factory is not configured",
		)
	}

	tx, err := beginSubmissionTransaction(ctx, s.transactions)
	if err != nil {
		return err
	}

	if err := tx.UpdateApplicationStatus(
		ctx,
		applicationID,
		StatusSubmissionInProgress,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf(
			"mark submission in progress: %w",
			err,
		)
	}

	if err := tx.CreateEvent(ctx, Event{
		ApplicationID: applicationID,
		Type:          EventApplicationSubmissionStarted,
		Metadata:      fmt.Sprintf("attempt_id=%s job_id=%d", attemptID, j.ID),
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf(
			"create submission started event: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf(
			"commit submission attempt: %w",
			err,
		)
	}

	// External submission happens only after the attempt is durably marked
	// as in-progress. If the process dies after this point, a retry will
	// require reconciliation instead of blindly submitting again.
	// We cannot roll back an external submission, so only update local
	// state after the external system confirms success.
	if err := s.submitter.Submit(ctx, *app, *j); err != nil {
		// The external submitter returned a definitive failure, so this
		// attempt did not cross the successful submission boundary.
		// Move the application back to READY_TO_APPLY and record the failure.
		tx, txErr := beginSubmissionTransaction(ctx, s.transactions)
		if txErr != nil {
			return fmt.Errorf(
				"submit application: %w; restore submission state: %v",
				err,
				txErr,
			)
		}

		if txErr := tx.UpdateApplicationStatus(
			ctx,
			applicationID,
			StatusReadyToApply,
		); txErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf(
				"submit application: %w; restore submission state: %v",
				err,
				txErr,
			)
		}

		if txErr := tx.CreateEvent(ctx, Event{
			ApplicationID: applicationID,
			Type:          EventApplicationSubmissionFailed,
			Metadata: fmt.Sprintf(
				"attempt_id=%s error=%s",
				attemptID,
				err.Error(),
			),
		}); txErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf(
				"submit application: %w; record failure: %v",
				err,
				txErr,
			)
		}

		if txErr := tx.Commit(); txErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf(
				"submit application: %w; commit failure state: %v",
				err,
				txErr,
			)
		}

		return fmt.Errorf("submit application: %w", err)
	}

	tx, err = beginSubmissionTransaction(ctx, s.transactions)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := tx.UpdateApplicationStatus(
		ctx,
		applicationID,
		StatusApplied,
	); err != nil {
		return fmt.Errorf(
			"mark application applied: %w",
			err,
		)
	}

	if err := tx.UpdateJobStatus(
		ctx,
		j.ID,
		job.StatusApplied,
	); err != nil {
		return fmt.Errorf(
			"mark job applied: %w",
			err,
		)
	}

	if err := tx.CreateEvent(ctx, Event{
		ApplicationID: applicationID,
		Type:          EventApplicationSubmitted,
		Metadata:      fmt.Sprintf("job_id=%d", j.ID),
	}); err != nil {
		return fmt.Errorf(
			"create submission event: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"commit submission transaction: %w",
			err,
		)
	}

	committed = true

	return nil
}
