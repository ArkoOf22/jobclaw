package application

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestSQLiteAnswerRepositoryCreateGetListAndUpdate(
	t *testing.T,
) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteAnswerRepository(db)
	ctx := context.Background()

	answer := CandidateAnswer{
		FieldKey:  "work_authorization",
		Question:  "Are you authorized to work in India?",
		Answer:    "Yes",
		ValueType: AnswerValueBoolean,
		Verified:  true,
		Notes:     "Candidate confirmed.",
	}

	if err := repo.Create(ctx, answer); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.GetByFieldKey(
		ctx,
		"work_authorization",
	)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatal("expected candidate answer")
	}

	if stored.FieldKey != answer.FieldKey {
		t.Fatalf(
			"field key = %q, want %q",
			stored.FieldKey,
			answer.FieldKey,
		)
	}

	if stored.Answer != "Yes" {
		t.Fatalf(
			"answer = %q, want Yes",
			stored.Answer,
		)
	}

	if stored.ValueType != AnswerValueBoolean {
		t.Fatalf(
			"value type = %q, want %q",
			stored.ValueType,
			AnswerValueBoolean,
		)
	}

	if !stored.Verified {
		t.Fatal("expected answer to be verified")
	}

	if stored.Notes != answer.Notes {
		t.Fatalf(
			"notes = %q, want %q",
			stored.Notes,
			answer.Notes,
		)
	}

	byID, err := repo.GetByID(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}

	if byID == nil {
		t.Fatal("expected answer by ID")
	}

	if byID.ID != stored.ID {
		t.Fatalf(
			"id = %d, want %d",
			byID.ID,
			stored.ID,
		)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Fatalf(
			"answer count = %d, want 1",
			len(list),
		)
	}

	stored.Answer = "No"
	stored.Verified = false
	stored.Notes = "Updated by candidate."

	if err := repo.Update(ctx, *stored); err != nil {
		t.Fatal(err)
	}

	updated, err := repo.GetByID(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}

	if updated == nil {
		t.Fatal("expected updated answer")
	}

	if updated.Answer != "No" {
		t.Fatalf(
			"updated answer = %q, want No",
			updated.Answer,
		)
	}

	if updated.Verified {
		t.Fatal("expected updated answer to be unverified")
	}
}

func TestSQLiteAnswerRepositoryRejectsInvalidInput(
	t *testing.T,
) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteAnswerRepository(db)
	ctx := context.Background()

	if err := repo.Create(
		ctx,
		CandidateAnswer{},
	); err == nil {
		t.Fatal("expected Create to reject missing field key")
	}

	if _, err := repo.GetByID(ctx, 0); err == nil {
		t.Fatal("expected GetByID to reject invalid ID")
	}

	if _, err := repo.GetByFieldKey(
		ctx,
		"",
	); err == nil {
		t.Fatal("expected GetByFieldKey to reject empty field key")
	}

	if err := repo.Update(
		ctx,
		CandidateAnswer{},
	); err == nil {
		t.Fatal("expected Update to reject invalid ID")
	}
}

func TestSQLiteAnswerRepositoryDuplicateFieldKey(
	t *testing.T,
) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteAnswerRepository(db)
	ctx := context.Background()

	answer := CandidateAnswer{
		FieldKey: "notice_period",
		Answer:   "30 days",
	}

	if err := repo.Create(ctx, answer); err != nil {
		t.Fatal(err)
	}

	if err := repo.Create(ctx, answer); err == nil {
		t.Fatal("expected duplicate field key to be rejected")
	}
}

func TestSQLiteAnswerRepositoryEmptyList(
	t *testing.T,
) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteAnswerRepository(db)

	answers, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if answers == nil {
		t.Fatal("expected non-nil empty slice")
	}

	if len(answers) != 0 {
		t.Fatalf(
			"answers = %d, want 0",
			len(answers),
		)
	}
}
