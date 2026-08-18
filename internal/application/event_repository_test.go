package application

import (
	"context"
	"testing"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

func TestSQLiteEventRepositoryCreateAndList(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	jobRepo := job.NewSQLiteRepository(db)

	if err := jobRepo.Upsert(ctx, job.Job{
		Source:      "test",
		ExternalID:  "event-test",
		Company:     "Setu",
		Title:       "Backend Engineer",
		Description: "Backend systems",
		Location:    "Bengaluru",
		URL:         "https://example.com/event-test",
	}); err != nil {
		t.Fatal(err)
	}

	j, err := jobRepo.GetBySourceExternalID(
		ctx,
		"test",
		"event-test",
	)
	if err != nil {
		t.Fatal(err)
	}

	if j == nil {
		t.Fatal("expected job")
	}

	appRepo := NewSQLiteRepository(db)

	if err := appRepo.Create(ctx, Application{
		JobID:  j.ID,
		Status: StatusDraft,
	}); err != nil {
		t.Fatal(err)
	}

	app, err := appRepo.GetByJobID(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}

	if app == nil {
		t.Fatal("expected application")
	}

	eventRepo := NewSQLiteEventRepository(db)

	events := []Event{
		{
			ApplicationID: app.ID,
			Type:          EventApplicationCreated,
			Metadata:      `{"source":"test"}`,
		},
		{
			ApplicationID: app.ID,
			Type:          EventResumeGenerationStarted,
			Metadata:      `{"attempt":1}`,
		},
		{
			ApplicationID: app.ID,
			Type:          EventResumeGenerationSucceeded,
			Metadata:      `{"attempt":2}`,
		},
	}

	for _, event := range events {
		if err := eventRepo.Create(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	stored, err := eventRepo.ListByApplicationID(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(stored) != len(events) {
		t.Fatalf("events = %d, want %d", len(stored), len(events))
	}

	for i, event := range stored {
		if event.ApplicationID != app.ID {
			t.Fatalf(
				"event[%d] application ID = %d, want %d",
				i,
				event.ApplicationID,
				app.ID,
			)
		}

		if event.Type != events[i].Type {
			t.Fatalf(
				"event[%d] type = %q, want %q",
				i,
				event.Type,
				events[i].Type,
			)
		}

		if event.Metadata != events[i].Metadata {
			t.Fatalf(
				"event[%d] metadata = %q, want %q",
				i,
				event.Metadata,
				events[i].Metadata,
			)
		}

		if event.CreatedAt.IsZero() {
			t.Fatalf("event[%d] created_at should be populated", i)
		}
	}
}

func TestSQLiteEventRepositoryRejectsInvalidInput(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteEventRepository(db)
	ctx := context.Background()

	if err := repo.Create(ctx, Event{}); err == nil {
		t.Fatal("expected Create to reject invalid application ID")
	}

	if err := repo.Create(ctx, Event{
		ApplicationID: 1,
	}); err == nil {
		t.Fatal("expected Create to reject missing event type")
	}

	if _, err := repo.ListByApplicationID(ctx, 0); err == nil {
		t.Fatal("expected ListByApplicationID to reject invalid application ID")
	}
}

func TestSQLiteEventRepositoryReturnsEmptyList(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteEventRepository(db)

	events, err := repo.ListByApplicationID(
		context.Background(),
		999,
	)
	if err != nil {
		t.Fatal(err)
	}

	if events == nil {
		t.Fatal("expected non-nil empty event list")
	}

	if len(events) != 0 {
		t.Fatalf("events = %d, want 0", len(events))
	}
}
