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
	transactions []*fakeSubmissionTransaction
	err          error
	index        int
}

func (f *fakeSubmissionTransactionFactory) BeginSubmissionTransaction(
	ctx context.Context,
) (SubmissionTransaction, error) {
	if f.err != nil {
		return nil, f.err
	}

	if f.index >= len(f.transactions) {
		return nil, errors.New("no fake submission transaction available")
	}

	tx := f.transactions[f.index]
	f.index++

	return tx, nil
}

type fakeApplicationSubmitter struct {
	err         error
	result      SubmissionResult
	submissions int
}

func (f *fakeApplicationSubmitter) Submit(
	ctx context.Context,
	app Application,
	j job.Job,
) (SubmissionResult, error) {
	f.submissions++

	if f.err != nil {
		if f.result != "" {
			return f.result, f.err
		}

		return SubmissionFailed, f.err
	}

	if f.result != "" {
		return f.result, nil
	}

	return SubmissionSucceeded, nil
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
	source string,
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

func newSubmissionFixture() (
	*fakeApplicationRepository,
	*fakeJobRepository,
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

	return applications, jobs
}

func TestApplicationSubmissionSucceeds(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	submitter := &fakeApplicationSubmitter{}

	attemptTx := &fakeSubmissionTransaction{}
	finalTx := &fakeSubmissionTransaction{}

	transactions := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			attemptTx,
			finalTx,
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(transactions)

	err := service.Submit(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}

	if submitter.submissions != 1 {
		t.Fatalf("submissions = %d, want 1", submitter.submissions)
	}

	if attemptTx.applicationStatus != StatusSubmissionInProgress {
		t.Fatalf(
			"attempt application status = %q, want %q",
			attemptTx.applicationStatus,
			StatusSubmissionInProgress,
		)
	}

	if !attemptTx.committed {
		t.Fatal("submission attempt transaction was not committed")
	}

	if attemptTx.rolledBack {
		t.Fatal("submission attempt transaction was rolled back")
	}

	if finalTx.applicationStatus != StatusApplied {
		t.Fatalf(
			"final application status = %q, want %q",
			finalTx.applicationStatus,
			StatusApplied,
		)
	}

	if finalTx.jobStatus != job.StatusApplied {
		t.Fatalf(
			"final job status = %q, want %q",
			finalTx.jobStatus,
			job.StatusApplied,
		)
	}

	if finalTx.event == nil {
		t.Fatal("submission event was not created")
	}

	if finalTx.event.Type != EventApplicationSubmitted {
		t.Fatalf(
			"event = %q, want %q",
			finalTx.event.Type,
			EventApplicationSubmitted,
		)
	}

	if !finalTx.committed {
		t.Fatal("final transaction was not committed")
	}

	if finalTx.rolledBack {
		t.Fatal("final transaction was rolled back")
	}
}

func TestApplicationSubmissionDoesNotChangeStateOnFailure(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	submitter := &fakeApplicationSubmitter{
		err: errors.New("external submission failed"),
	}

	attemptTx := &fakeSubmissionTransaction{}
	failureTx := &fakeSubmissionTransaction{}

	transactions := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			attemptTx,
			failureTx,
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(transactions)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected submission failure")
	}

	if submitter.submissions != 1 {
		t.Fatalf("submissions = %d, want 1", submitter.submissions)
	}

	if attemptTx.applicationStatus != StatusSubmissionInProgress {
		t.Fatalf(
			"attempt status = %q, want %q",
			attemptTx.applicationStatus,
			StatusSubmissionInProgress,
		)
	}

	if !attemptTx.committed {
		t.Fatal("attempt transaction was not committed")
	}

	if failureTx.applicationStatus != StatusReadyToApply {
		t.Fatalf(
			"failure status = %q, want %q",
			failureTx.applicationStatus,
			StatusReadyToApply,
		)
	}

	if failureTx.event == nil {
		t.Fatal("submission failure event was not created")
	}

	if failureTx.event.Type != EventApplicationSubmissionFailed {
		t.Fatalf(
			"event = %q, want %q",
			failureTx.event.Type,
			EventApplicationSubmissionFailed,
		)
	}

	if !failureTx.committed {
		t.Fatal("failure-state transaction was not committed")
	}

	app, err := applications.GetByID(context.Background(), 100)
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

	storedJob, err := jobs.GetByID(context.Background(), 10)
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

func TestApplicationSubmissionRollsBackLocalChanges(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	submitter := &fakeApplicationSubmitter{}

	attemptTx := &fakeSubmissionTransaction{}

	finalTx := &fakeSubmissionTransaction{
		updateJobErr: errors.New("job status update failed"),
	}

	transactions := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			attemptTx,
			finalTx,
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(transactions)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected transaction failure")
	}

	if submitter.submissions != 1 {
		t.Fatalf("submissions = %d, want 1", submitter.submissions)
	}

	if !attemptTx.committed {
		t.Fatal("submission attempt transaction was not committed")
	}

	if finalTx.applicationStatus != StatusApplied {
		t.Fatalf(
			"final transaction application status = %q, want %q",
			finalTx.applicationStatus,
			StatusApplied,
		)
	}

	if finalTx.committed {
		t.Fatal("failed final transaction was committed")
	}

	if !finalTx.rolledBack {
		t.Fatal("failed final transaction was not rolled back")
	}
}

func TestApplicationSubmissionAmbiguousLeavesApplicationInProgress(
	t *testing.T,
) {
	applications, jobs := newSubmissionFixture()
	events := &fakeEventRepository{}

	submitter := &fakeApplicationSubmitter{
		result: SubmissionAmbiguous,
		err:    errors.New("network timeout after remote acceptance"),
	}

	startTx := &fakeSubmissionTransaction{}
	factory := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			startTx,
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		events,
		submitter,
	)
	service.SetTransactionFactory(factory)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected ambiguous submission error")
	}

	if submitter.submissions != 1 {
		t.Fatalf(
			"submissions = %d, want 1",
			submitter.submissions,
		)
	}

	if len(events.events) != 0 {
		t.Fatalf(
			"events = %d, want 0 for fake repository",
			len(events.events),
		)
	}

	if startTx.applicationStatus != StatusSubmissionInProgress {
		t.Fatalf(
			"transaction application status = %q, want %q",
			startTx.applicationStatus,
			StatusSubmissionInProgress,
		)
	}

	if !startTx.committed {
		t.Fatal("submission-start transaction was not committed")
	}

	if startTx.rolledBack {
		t.Fatal("submission-start transaction was rolled back")
	}
}

func TestApplicationSubmissionRejectsRetryAfterAmbiguousAttempt(
	t *testing.T,
) {
	applications, jobs := newSubmissionFixture()
	events := &fakeEventRepository{}

	submitter := &fakeApplicationSubmitter{
		result: SubmissionAmbiguous,
		err:    errors.New("remote outcome unknown"),
	}

	startTx := &fakeSubmissionTransaction{}
	factory := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			startTx,
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		events,
		submitter,
	)
	service.SetTransactionFactory(factory)

	firstErr := service.Submit(
		context.Background(),
		100,
	)
	if firstErr == nil {
		t.Fatal("first submission should be ambiguous")
	}

	secondErr := service.Submit(
		context.Background(),
		100,
	)
	if secondErr == nil {
		t.Fatal("retry after ambiguous submission should be rejected")
	}

	if submitter.submissions != 1 {
		t.Fatalf(
			"submissions = %d, want 1; ambiguous retry must not resubmit",
			submitter.submissions,
		)
	}

	if factory.index != 1 {
		t.Fatalf(
			"transaction factory index = %d, want 1; retry must not start another transaction",
			factory.index,
		)
	}

	if startTx.applicationStatus != StatusSubmissionInProgress {
		t.Fatalf(
			"submission transaction application status = %q, want %q",
			startTx.applicationStatus,
			StatusSubmissionInProgress,
		)
	}
}

func TestApplicationSubmissionRejectsNonReadyApplication(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	applications.applications[100].Status = StatusDraft

	submitter := &fakeApplicationSubmitter{}
	transactions := &fakeSubmissionTransactionFactory{}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(transactions)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected non-ready application to be rejected")
	}

	if submitter.submissions != 0 {
		t.Fatalf("submissions = %d, want 0", submitter.submissions)
	}

	if transactions.index != 0 {
		t.Fatalf(
			"transactions started = %d, want 0",
			transactions.index,
		)
	}
}

func TestApplicationSubmissionRejectsNonApprovedJob(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	jobs.jobs[10].Status = job.StatusDiscovered

	submitter := &fakeApplicationSubmitter{}
	transactions := &fakeSubmissionTransactionFactory{}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(transactions)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected non-approved job to be rejected")
	}

	if submitter.submissions != 0 {
		t.Fatalf("submissions = %d, want 0", submitter.submissions)
	}

	if transactions.index != 0 {
		t.Fatalf(
			"transactions started = %d, want 0",
			transactions.index,
		)
	}
}

// An ambiguous outcome must be identifiable via errors.Is rather than by string
// matching. Callers that cannot distinguish it will report "nothing was
// submitted" for a request that may have reached the employer, which is how
// duplicate applications get sent.
func TestAmbiguousSubmissionErrorIsIdentifiable(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	submitter := &fakeApplicationSubmitter{
		result: SubmissionAmbiguous,
		err:    errors.New("network timeout after remote acceptance"),
	}

	factory := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			{},
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(factory)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected an error for an ambiguous submission")
	}

	if !errors.Is(err, ErrSubmissionAmbiguous) {
		t.Fatalf(
			"error %v does not wrap ErrSubmissionAmbiguous",
			err,
		)
	}
}

// Retrying an application already locked in SUBMISSION_IN_PROGRESS is also an
// ambiguous case and must be reported as such, not as a generic failure.
func TestRetryAfterAmbiguousAttemptIsIdentifiable(t *testing.T) {
	applications, jobs := newSubmissionFixture()

	// Simulate an application already locked by a prior unresolved attempt.
	applications.applications[100].Status = StatusSubmissionInProgress

	submitter := &fakeApplicationSubmitter{
		result: SubmissionSucceeded,
	}

	factory := &fakeSubmissionTransactionFactory{
		transactions: []*fakeSubmissionTransaction{
			{},
		},
	}

	service := NewApplicationSubmissionService(
		applications,
		jobs,
		&fakeEventRepository{},
		submitter,
	)
	service.SetTransactionFactory(factory)

	err := service.Submit(context.Background(), 100)
	if err == nil {
		t.Fatal("expected a retry of a locked application to be rejected")
	}

	if !errors.Is(err, ErrSubmissionAmbiguous) {
		t.Fatalf(
			"error %v does not wrap ErrSubmissionAmbiguous",
			err,
		)
	}

	if submitter.submissions != 0 {
		t.Fatalf(
			"submissions = %d, want 0; a locked application must not resubmit",
			submitter.submissions,
		)
	}
}
