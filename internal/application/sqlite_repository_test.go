package application

import (
	"context"
	"testing"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

func TestSQLiteRepositoryCreateGetAndUpdateStatus(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	jobRepo := job.NewSQLiteRepository(db)

	ctx := context.Background()

	if err := jobRepo.Upsert(ctx, job.Job{
		Source:      "test",
		ExternalID:  "application-test",
		Company:     "Setu",
		Title:       "Backend Engineer",
		Description: "Backend systems",
		Location:    "Bengaluru",
		URL:         "https://example.com/application-test",
	}); err != nil {
		t.Fatal(err)
	}

	j, err := jobRepo.GetBySourceExternalID(
		ctx,
		"test",
		"application-test",
	)
	if err != nil {
		t.Fatal(err)
	}

	if j == nil {
		t.Fatal("expected job")
	}

	repo := NewSQLiteRepository(db)

	app := Application{
		JobID:                  j.ID,
		Status:                 StatusDraft,
		TailoredResumePath:     "artifacts/resume.pdf",
		CoverLetterPath:        "artifacts/cover-letter.md",
		ReferralMessagePath:    "artifacts/referral.md",
		ApplicationAnswersPath: "artifacts/answers.json",
	}

	if err := repo.Create(ctx, app); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.GetByJobID(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatal("expected application")
	}

	if stored.JobID != j.ID {
		t.Fatalf("job ID = %d, want %d", stored.JobID, j.ID)
	}

	if stored.Status != StatusDraft {
		t.Fatalf("status = %q, want %q", stored.Status, StatusDraft)
	}

	if stored.TailoredResumePath != app.TailoredResumePath {
		t.Fatalf(
			"resume path = %q, want %q",
			stored.TailoredResumePath,
			app.TailoredResumePath,
		)
	}

	if stored.CoverLetterPath != app.CoverLetterPath {
		t.Fatalf(
			"cover letter path = %q, want %q",
			stored.CoverLetterPath,
			app.CoverLetterPath,
		)
	}

	if err := repo.UpdateStatus(
		ctx,
		stored.ID,
		StatusReadyForReview,
	); err != nil {
		t.Fatal(err)
	}

	updated, err := repo.GetByJobID(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}

	if updated == nil {
		t.Fatal("expected updated application")
	}

	if updated.Status != StatusReadyForReview {
		t.Fatalf(
			"status = %q, want %q",
			updated.Status,
			StatusReadyForReview,
		)
	}
}

func TestSQLiteRepositoryRejectsInvalidIDs(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteRepository(db)
	ctx := context.Background()

	if err := repo.Create(ctx, Application{}); err == nil {
		t.Fatal("expected Create to reject invalid job ID")
	}

	if _, err := repo.GetByJobID(ctx, 0); err == nil {
		t.Fatal("expected GetByJobID to reject invalid job ID")
	}

	if err := repo.UpdateStatus(ctx, 0, StatusDraft); err == nil {
		t.Fatal("expected UpdateStatus to reject invalid application ID")
	}
}
