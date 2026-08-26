package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"jobclaw/internal/job"
)

type SubmissionResult string

const (
	SubmissionSucceeded SubmissionResult = "SUCCEEDED"
	SubmissionFailed    SubmissionResult = "FAILED"
	SubmissionAmbiguous SubmissionResult = "AMBIGUOUS"
)

// ErrSubmissionAmbiguous marks the case where the request may have reached the
// external system but the outcome could not be confirmed. The application may
// or may not have been submitted.
//
// This must be distinguishable by callers rather than inferred from an error
// string: reporting an ambiguous outcome as a plain failure invites the operator
// to resubmit, which is exactly how duplicate applications happen.
var ErrSubmissionAmbiguous = errors.New(
	"submission outcome is ambiguous",
)

type ApplicationSubmitter interface {
	Submit(
		ctx context.Context,
		app Application,
		j job.Job,
	) (SubmissionResult, error)
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
			"application %d has an unresolved submission attempt; reconciliation is required: %w",
			applicationID,
			ErrSubmissionAmbiguous,
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
	result, submitErr := s.submitter.Submit(ctx, *app, *j)

	if result == SubmissionAmbiguous {
		return fmt.Errorf(
			"submission attempt %s could not be confirmed; reconciliation is required: %w",
			attemptID,
			ErrSubmissionAmbiguous,
		)
	}

	if result == SubmissionFailed {
		if submitErr == nil {
			submitErr = fmt.Errorf("external submission failed")
		}

		// A definitive external failure means the application can safely
		// return to READY_TO_APPLY. Keep the state transition and failure
		// event in one local transaction.
		if s.transactions == nil {
			if s.events != nil {
				_ = s.events.Create(ctx, Event{
					ApplicationID: applicationID,
					Type:          EventApplicationSubmissionFailed,
					Metadata: fmt.Sprintf(
						"attempt_id=%s error=%s",
						attemptID,
						submitErr.Error(),
					),
				})
			}

			return fmt.Errorf("submit application: %w", submitErr)
		}

		failureTx, txErr := beginSubmissionTransaction(
			ctx,
			s.transactions,
		)
		if txErr != nil {
			return fmt.Errorf(
				"submit application: %w; restore submission state: %v",
				submitErr,
				txErr,
			)
		}

		failureCommitted := false
		defer func() {
			if !failureCommitted {
				_ = failureTx.Rollback()
			}
		}()

		if txErr := failureTx.UpdateApplicationStatus(
			ctx,
			applicationID,
			StatusReadyToApply,
		); txErr != nil {
			return fmt.Errorf(
				"submit application: %w; restore submission state: %v",
				submitErr,
				txErr,
			)
		}

		if txErr := failureTx.CreateEvent(ctx, Event{
			ApplicationID: applicationID,
			Type:          EventApplicationSubmissionFailed,
			Metadata: fmt.Sprintf(
				"attempt_id=%s error=%s",
				attemptID,
				submitErr.Error(),
			),
		}); txErr != nil {
			return fmt.Errorf(
				"submit application: %w; record submission failure: %v",
				submitErr,
				txErr,
			)
		}

		if txErr := failureTx.Commit(); txErr != nil {
			return fmt.Errorf(
				"submit application: %w; commit failure state: %v",
				submitErr,
				txErr,
			)
		}

		failureCommitted = true

		return fmt.Errorf("submit application: %w", submitErr)
	}

	if result != SubmissionSucceeded {
		if submitErr != nil {
			return fmt.Errorf(
				"submit application: unexpected submission result %q: %w",
				result,
				submitErr,
			)
		}

		return fmt.Errorf(
			"submit application: unexpected submission result %q",
			result,
		)
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
