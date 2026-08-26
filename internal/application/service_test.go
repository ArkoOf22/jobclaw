package application

import (
	"context"
	"errors"
	"os"
	"testing"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

func setupApplicationServiceTest(t *testing.T) (
	*database.DB,
	*job.SQLiteRepository,
	*SQLiteRepository,
	*Service,
) {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.MigrateEmbedded(); err != nil {
		db.Close()
		t.Fatal(err)
	}

	jobRepo := job.NewSQLiteRepository(db)
	appRepo := NewSQLiteRepository(db)
	resume := NewMockResumeGenerator()

	events := NewSQLiteEventRepository(db)
	return db, jobRepo, appRepo, NewService(jobRepo, appRepo, events, resume)
}

func createTestJob(
	t *testing.T,
	ctx context.Context,
	repo *job.SQLiteRepository,
	status job.Status,
	externalID string,
) int64 {
	t.Helper()

	err := repo.Upsert(ctx, job.Job{
		Source:      "test",
		ExternalID:  externalID,
		Company:     "Setu",
		Title:       "SDE II",
		Description: "Backend Engineer",
		Location:    "Bengaluru",
		URL:         "https://example.com/" + externalID,
	})

	if err != nil {
		t.Fatal(err)
	}

	j, err := repo.GetBySourceExternalID(ctx, "test", externalID)
	if err != nil {
		t.Fatal(err)
	}

	if j == nil {
		t.Fatal("expected job")
	}

	if err := repo.UpdateStatus(ctx, j.ID, status); err != nil {
		t.Fatal(err)
	}

	return j.ID
}

func TestCreateForApprovedJob(t *testing.T) {
	db, jobRepo, _, service := setupApplicationServiceTest(t)
	defer db.Close()

	useTempWorkingDir(t)

	ctx := context.Background()

	jobID := createTestJob(
		t,
		ctx,
		jobRepo,
		job.StatusApproved,
		"approved-job",
	)

	app, err := service.CreateForApprovedJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if app == nil {
		t.Fatal("expected application")
	}

	if app.JobID != jobID {
		t.Fatalf("job ID = %d, want %d", app.JobID, jobID)
	}

	if app.Status != StatusDraft {
		t.Fatalf("status = %s, want %s", app.Status, StatusDraft)
	}
}

func TestCreateForNonApprovedJobRejected(t *testing.T) {
	tests := []struct {
		name   string
		status job.Status
	}{
		{
			name:   "discovered",
			status: job.StatusDiscovered,
		},
		{
			name:   "scored",
			status: job.StatusScored,
		},
		{
			name:   "shortlisted",
			status: job.StatusShortlisted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, jobRepo, _, service := setupApplicationServiceTest(t)
			defer db.Close()

			ctx := context.Background()

			jobID := createTestJob(
				t,
				ctx,
				jobRepo,
				tt.status,
				"rejected-"+tt.name,
			)

			app, err := service.CreateForApprovedJob(ctx, jobID)

			if err == nil {
				t.Fatal("expected error")
			}

			if app != nil {
				t.Fatal("expected nil application")
			}
		})
	}
}

func TestCreateForApprovedJobDoesNotDuplicate(t *testing.T) {
	db, jobRepo, appRepo, service := setupApplicationServiceTest(t)
	defer db.Close()

	useTempWorkingDir(t)

	ctx := context.Background()

	jobID := createTestJob(
		t,
		ctx,
		jobRepo,
		job.StatusApproved,
		"duplicate-test",
	)

	first, err := service.CreateForApprovedJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	second, err := service.CreateForApprovedJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Fatalf(
			"duplicate application created: first=%d second=%d",
			first.ID,
			second.ID,
		)
	}

	app, err := appRepo.GetByJobID(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if app == nil {
		t.Fatal("expected application")
	}

	if app.ID != first.ID {
		t.Fatalf("stored application ID = %d, want %d", app.ID, first.ID)
	}
}

// useTempWorkingDir points the process at a throwaway directory for the test.
//
// Application workspaces are created relative to the working directory, so
// without this, tests write data/applications/... into the source tree. Leftover
// files there can mask or unmask failures on later runs.
func useTempWorkingDir(t *testing.T) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir to temp working directory: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
}

// flakyResumeGenerator fails a configurable number of times before succeeding,
// so a transient LLM outage can be simulated.
type flakyResumeGenerator struct {
	failures int
	calls    int
	delegate ResumeGenerator
}

func (g *flakyResumeGenerator) GenerateTailoredResume(
	ctx context.Context,
	jobID int64,
	outputPath string,
) error {
	g.calls++

	if g.calls <= g.failures {
		return errors.New("simulated LLM outage")
	}

	return g.delegate.GenerateTailoredResume(ctx, jobID, outputPath)
}

// Creating an application is a local operation. It must not require the resume
// generator, so that a missing or unreachable LLM cannot block workspace setup.
func TestCreateForApprovedJobWithoutResumeGenerator(t *testing.T) {
	db, jobRepo, _, _ := setupApplicationServiceTest(t)
	defer db.Close()

	useTempWorkingDir(t)

	ctx := context.Background()

	service := NewService(
		jobRepo,
		NewSQLiteRepository(db),
		NewSQLiteEventRepository(db),
		nil,
	)

	jobID := createTestJob(
		t,
		ctx,
		jobRepo,
		job.StatusApproved,
		"no-generator-job",
	)

	app, err := service.CreateForApprovedJob(ctx, jobID)
	if err != nil {
		t.Fatalf("create application without generator: %v", err)
	}

	if app == nil {
		t.Fatal("expected application")
	}

	if app.Status != StatusDraft {
		t.Fatalf("status = %s, want %s", app.Status, StatusDraft)
	}

	if app.TailoredResumePath != "" {
		t.Fatalf(
			"resume path = %q, want empty before generation",
			app.TailoredResumePath,
		)
	}
}

// Regression test for the original defect: creation and resume generation were
// fused, so a generation failure left a committed application that could never
// acquire a resume. The retry matched the "already exists" branch and reported
// success while doing nothing.
func TestEnsureTailoredResumeRecoversAfterFailure(t *testing.T) {
	db, jobRepo, appRepo, _ := setupApplicationServiceTest(t)
	defer db.Close()

	ctx := context.Background()

	useTempWorkingDir(t)

	generator := &flakyResumeGenerator{
		failures: 1,
		delegate: NewMockResumeGenerator(),
	}

	service := NewService(
		jobRepo,
		appRepo,
		NewSQLiteEventRepository(db),
		generator,
	)

	jobID := createTestJob(
		t,
		ctx,
		jobRepo,
		job.StatusApproved,
		"recovering-job",
	)

	if _, err := service.CreateForApprovedJob(ctx, jobID); err != nil {
		t.Fatalf("create application: %v", err)
	}

	// First attempt fails, mirroring an LLM outage.
	if _, err := service.EnsureTailoredResume(ctx, jobID); err == nil {
		t.Fatal("expected first resume attempt to fail")
	}

	stored, err := appRepo.GetByJobID(ctx, jobID)
	if err != nil {
		t.Fatalf("load application after failure: %v", err)
	}

	if stored == nil {
		t.Fatal("application must survive a resume failure")
	}

	if stored.TailoredResumePath != "" {
		t.Fatalf(
			"resume path = %q, want empty after failure",
			stored.TailoredResumePath,
		)
	}

	// The retry must actually generate, not silently no-op.
	resumePath, err := service.EnsureTailoredResume(ctx, jobID)
	if err != nil {
		t.Fatalf("retry should succeed: %v", err)
	}

	if resumePath == "" {
		t.Fatal("expected a resume path from the retry")
	}

	if generator.calls != 2 {
		t.Fatalf(
			"generator calls = %d, want 2; the retry must regenerate",
			generator.calls,
		)
	}

	info, err := os.Stat(resumePath)
	if err != nil {
		t.Fatalf("stat generated resume: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("generated resume is empty")
	}

	stored, err = appRepo.GetByJobID(ctx, jobID)
	if err != nil {
		t.Fatalf("load application after retry: %v", err)
	}

	if stored.TailoredResumePath != resumePath {
		t.Fatalf(
			"stored resume path = %q, want %q",
			stored.TailoredResumePath,
			resumePath,
		)
	}
}

// A second call must not regenerate when a valid artifact already exists.
func TestEnsureTailoredResumeIsIdempotent(t *testing.T) {
	db, jobRepo, appRepo, _ := setupApplicationServiceTest(t)
	defer db.Close()

	ctx := context.Background()

	useTempWorkingDir(t)

	generator := &flakyResumeGenerator{
		delegate: NewMockResumeGenerator(),
	}

	service := NewService(
		jobRepo,
		appRepo,
		NewSQLiteEventRepository(db),
		generator,
	)

	jobID := createTestJob(
		t,
		ctx,
		jobRepo,
		job.StatusApproved,
		"idempotent-job",
	)

	if _, err := service.CreateForApprovedJob(ctx, jobID); err != nil {
		t.Fatalf("create application: %v", err)
	}

	first, err := service.EnsureTailoredResume(ctx, jobID)
	if err != nil {
		t.Fatalf("first generation: %v", err)
	}

	second, err := service.EnsureTailoredResume(ctx, jobID)
	if err != nil {
		t.Fatalf("second generation: %v", err)
	}

	if first != second {
		t.Fatalf("path changed between calls: %q then %q", first, second)
	}

	if generator.calls != 1 {
		t.Fatalf(
			"generator calls = %d, want 1; an existing resume must be reused",
			generator.calls,
		)
	}
}

// A recorded path whose file has gone missing must be regenerated rather than
// trusted, otherwise readiness checks pass against an artifact that is not there.
func TestEnsureTailoredResumeRegeneratesMissingFile(t *testing.T) {
	db, jobRepo, appRepo, _ := setupApplicationServiceTest(t)
	defer db.Close()

	ctx := context.Background()

	useTempWorkingDir(t)

	generator := &flakyResumeGenerator{
		delegate: NewMockResumeGenerator(),
	}

	service := NewService(
		jobRepo,
		appRepo,
		NewSQLiteEventRepository(db),
		generator,
	)

	jobID := createTestJob(
		t,
		ctx,
		jobRepo,
		job.StatusApproved,
		"missing-artifact-job",
	)

	if _, err := service.CreateForApprovedJob(ctx, jobID); err != nil {
		t.Fatalf("create application: %v", err)
	}

	resumePath, err := service.EnsureTailoredResume(ctx, jobID)
	if err != nil {
		t.Fatalf("first generation: %v", err)
	}

	if err := os.Remove(resumePath); err != nil {
		t.Fatalf("remove generated resume: %v", err)
	}

	if _, err := service.EnsureTailoredResume(ctx, jobID); err != nil {
		t.Fatalf("regeneration after deletion: %v", err)
	}

	if generator.calls != 2 {
		t.Fatalf(
			"generator calls = %d, want 2; a missing artifact must be regenerated",
			generator.calls,
		)
	}
}
