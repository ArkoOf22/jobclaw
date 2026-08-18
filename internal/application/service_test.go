package application

import (
	"context"
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
