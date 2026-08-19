package application

import (
	"context"
	"errors"
	"testing"

	"jobclaw/internal/job"
)

type fakeSubmissionTransaction struct {
	applicationStatus Status
	jobStatus         job.Status
	event             *Event

	updateApplicationErr error
	updateJobErr         error
	createEventErr       error
	commitErr            error

	committed  bool
	rolledBack bool
}

func (f *fakeSubmissionTransaction) UpdateApplicationStatus(
	ctx context.Context,
	id int64,
	status Status,
) error {
	if f.updateApplicationErr != nil {
		return f.updateApplicationErr
	}

	f.applicationStatus = status
	return nil
}

func (f *fakeSubmissionTransaction) UpdateJobStatus(
	ctx context.Context,
	id int64,
	status job.Status,
) error {
	if f.updateJobErr != nil {
		return f.updateJobErr
	}

	f.jobStatus = status
	return nil
}

func (f *fakeSubmissionTransaction) CreateEvent(
	ctx context.Context,
	event Event,
) error {
	if f.createEventErr != nil {
		return f.createEventErr
	}

	copy := event
	f.event = &copy
	return nil
}

func (f *fakeSubmissionTransaction) Commit() error {
	if f.commitErr != nil {
		return f.commitErr
	}

	f.committed = true
	return nil
}

func (f *fakeSubmissionTransaction) Rollback() error {
	f.rolledBack = true
	return nil
}

type fakeSubmissionTransactionFactory struct {
	tx  *fakeSubmissionTransaction
	err error
}

func (f *fakeSubmissionTransactionFactory) BeginSubmissionTransaction(
	ctx context.Context,
) (SubmissionTransaction, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.tx, nil
}

type fakeApplicationSubmitter struct {
	err         error
	submissions int
}

func (f *fakeApplicationSubmitter) Submit(
	ctx context.Context,
	app Application,
	j job.Job,
) error {
	f.submissions++

	return f.err
}

type fakeJobRepository struct {
	jobs map[int64]*job.Job
}

func (f *fakeJobRepository) Upsert(
	ctx context.Context,
	j job.Job,
) error {
	if f.jobs == nil {
		f.jobs = make(map[int64]*job.Job)
	}

	copy := j
	f.jobs[j.ID] = &copy

	return nil
}

func (f *fakeJobRepository) GetBySourceExternalID(
	ctx context.Context,
	source,
	externalID string,
) (*job.Job, error) {
	for _, j := range f.jobs {
		if j.Source == source && j.ExternalID == externalID {
			copy := *j
			return &copy, nil
		}
	}

	return nil, nil
}

func (f *fakeJobRepository) GetByID(
	ctx context.Context,
	id int64,
) (*job.Job, error) {
	j := f.jobs[id]

	if j == nil {
		return nil, nil
	}

	copy := *j
	return &copy, nil
}

func (f *fakeJobRepository) List(
	ctx context.Context,
	limit int,
) ([]job.Job, error) {
	var result []job.Job

	for _, j := range f.jobs {
		result = append(result, *j)
	}

	return result, nil
}

func (f *fakeJobRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status job.Status,
) error {
	j := f.jobs[id]

	if j == nil {
		return errors.New("job not found")
	}

	j.Status = status
	return nil
}

func TestApplicationSubmissionSucceeds(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:                 100,
				JobID:              10,
				Status:             StatusReadyToApply,
				TailoredResumePath: "resume.txt",
			},
		},
	}

	jobs := &fakeJobRepository{
		jobs: map[int64]*job.Job{
			10: {
				ID:     10,
				Status: job.StatusApproved,
			},
		},
	}

	events := &fakeEventRepository{}
	submitter := &fakeApplicationSubmitter{}

	tx := &fakeSubmissionTransaction{}
	transactions := &fakeSubmissionTransactionFactory{
		tx: tx,
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		events,
		submitter,
	)
	service.SetTransactionFactory(transactions)

	err := service.Submit(
		context.Background(),
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	if tx.applicationStatus != StatusApplied {
		t.Fatalf(
			"transaction application status = %q, want %q",
			tx.applicationStatus,
			StatusApplied,
		)
	}

	if tx.jobStatus != job.StatusApplied {
		t.Fatalf(
			"transaction job status = %q, want %q",
			tx.jobStatus,
			job.StatusApplied,
		)
	}

	if !tx.committed {
		t.Fatal("transaction was not committed")
	}

	if tx.rolledBack {
		t.Fatal("transaction was rolled back after successful commit")
	}

	if submitter.submissions != 1 {
		t.Fatalf(
			"submissions = %d, want 1",
			submitter.submissions,
		)
	}

	if tx.event == nil {
		t.Fatal("submission event was not created")
	}

	if tx.event.Type != EventApplicationSubmitted {
		t.Fatalf(
			"event = %q, want %q",
			tx.event.Type,
			EventApplicationSubmitted,
		)
	}
}

func TestApplicationSubmissionDoesNotChangeStateOnFailure(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:     100,
				JobID:  10,
				Status: StatusReadyToApply,
			},
		},
	}

	jobs := &fakeJobRepository{
		jobs: map[int64]*job.Job{
			10: {
				ID:     10,
				Status: job.StatusApproved,
			},
		},
	}

	events := &fakeEventRepository{}
	submitter := &fakeApplicationSubmitter{
		err: errors.New("submission failed"),
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		events,
		submitter,
	)

	err := service.Submit(
		context.Background(),
		100,
	)
	if err == nil {
		t.Fatal("expected submission failure")
	}

	app, err := applications.GetByID(
		context.Background(),
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	if app.Status != StatusReadyToApply {
		t.Fatalf(
			"application status = %q, want %q",
			app.Status,
			StatusReadyToApply,
		)
	}

	storedJob, err := jobs.GetByID(
		context.Background(),
		10,
	)
	if err != nil {
		t.Fatal(err)
	}

	if storedJob.Status != job.StatusApproved {
		t.Fatalf(
			"job status = %q, want %q",
			storedJob.Status,
			job.StatusApproved,
		)
	}

	if len(events.events) != 1 {
		t.Fatalf(
			"events = %d, want 1",
			len(events.events),
		)
	}

	if events.events[0].Type != EventApplicationSubmissionFailed {
		t.Fatalf(
			"event = %q, want %q",
			events.events[0].Type,
			EventApplicationSubmissionFailed,
		)
	}
}

func TestApplicationSubmissionRollsBackLocalChanges(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:     100,
				JobID:  10,
				Status: StatusReadyToApply,
			},
		},
	}

	jobs := &fakeJobRepository{
		jobs: map[int64]*job.Job{
			10: {
				ID:     10,
				Status: job.StatusApproved,
			},
		},
	}

	submitter := &fakeApplicationSubmitter{}

	tx := &fakeSubmissionTransaction{
		updateJobErr: errors.New("job status update failed"),
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(
		&fakeSubmissionTransactionFactory{tx: tx},
	)

	err := service.Submit(
		context.Background(),
		100,
	)
	if err == nil {
		t.Fatal("expected transaction failure")
	}

	if tx.applicationStatus != StatusApplied {
		t.Fatalf(
			"transaction application status = %q, want %q before rollback",
			tx.applicationStatus,
			StatusApplied,
		)
	}

	if !tx.rolledBack {
		t.Fatal("transaction was not rolled back")
	}

	if tx.committed {
		t.Fatal("failed transaction was committed")
	}

	app, err := applications.GetByID(
		context.Background(),
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	if app.Status != StatusReadyToApply {
		t.Fatalf(
			"application status = %q, want %q",
			app.Status,
			StatusReadyToApply,
		)
	}

	storedJob, err := jobs.GetByID(
		context.Background(),
		10,
	)
	if err != nil {
		t.Fatal(err)
	}

	if storedJob.Status != job.StatusApproved {
		t.Fatalf(
			"job status = %q, want %q",
			storedJob.Status,
			job.StatusApproved,
		)
	}
}

func TestApplicationSubmissionRejectsNonReadyApplication(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:     100,
				JobID:  10,
				Status: StatusDraft,
			},
		},
	}

	jobs := &fakeJobRepository{
		jobs: map[int64]*job.Job{
			10: {
				ID:     10,
				Status: job.StatusApproved,
			},
		},
	}

	submitter := &fakeApplicationSubmitter{}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)

	err := service.Submit(
		context.Background(),
		100,
	)
	if err == nil {
		t.Fatal("expected non-ready application to be rejected")
	}

	if submitter.submissions != 0 {
		t.Fatalf(
			"submissions = %d, want 0",
			submitter.submissions,
		)
	}
}

func TestApplicationSubmissionRejectsNonApprovedJob(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:     100,
				JobID:  10,
				Status: StatusReadyToApply,
			},
		},
	}

	jobs := &fakeJobRepository{
		jobs: map[int64]*job.Job{
			10: {
				ID:     10,
				Status: job.StatusShortlisted,
			},
		},
	}

	submitter := &fakeApplicationSubmitter{}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)

	err := service.Submit(
		context.Background(),
		100,
	)
	if err == nil {
		t.Fatal("expected non-approved job to be rejected")
	}

	if submitter.submissions != 0 {
		t.Fatalf(
			"submissions = %d, want 0",
			submitter.submissions,
		)
	}
}
