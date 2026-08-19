package application

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestSQLiteQuestionRepositoryCreateGetListAndUpdate(
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

	ctx := context.Background()

	questionRepo := NewSQLiteQuestionRepository(db)

	_, err = db.ExecContext(ctx, `
		INSERT INTO job_sources (
			name,
			base_url
		)
		VALUES (?, ?)
	`,
		"test",
		"https://example.com",
	)
	if err != nil {
		t.Fatal(err)
	}

	var sourceID int64

	if err := db.QueryRowContext(
		ctx,
		`SELECT id FROM job_sources WHERE name = ?`,
		"test",
	).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO jobs (
			source_id,
			external_id,
			company,
			title,
			description,
			location,
			url
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		sourceID,
		"question-test",
		"Setu",
		"Backend Engineer",
		"Backend systems",
		"Bengaluru",
		"https://example.com/question-test",
	)
	if err != nil {
		t.Fatal(err)
	}

	var jobID int64

	if err := db.QueryRowContext(
		ctx,
		`SELECT id FROM jobs WHERE external_id = ?`,
		"question-test",
	).Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO applications (
			job_id,
			status
		)
		VALUES (?, ?)
	`,
		jobID,
		StatusDraft,
	)
	if err != nil {
		t.Fatal(err)
	}

	var applicationID int64

	if err := db.QueryRowContext(
		ctx,
		`SELECT id FROM applications WHERE job_id = ?`,
		jobID,
	).Scan(&applicationID); err != nil {
		t.Fatal(err)
	}

	question := ApplicationQuestion{
		ApplicationID: applicationID,
		Question:      "Are you authorized to work in India?",
		FieldKey:      "work_authorization",
		Status:        QuestionNeedsReview,
	}

	if err := questionRepo.Create(ctx, question); err != nil {
		t.Fatal(err)
	}

	questions, err := questionRepo.ListByApplicationID(
		ctx,
		applicationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(questions))
	}

	stored := questions[0]

	if stored.Question != question.Question {
		t.Fatalf(
			"question = %q, want %q",
			stored.Question,
			question.Question,
		)
	}

	if stored.FieldKey != "work_authorization" {
		t.Fatalf(
			"field key = %q, want work_authorization",
			stored.FieldKey,
		)
	}

	if stored.Status != QuestionNeedsReview {
		t.Fatalf(
			"status = %q, want %q",
			stored.Status,
			QuestionNeedsReview,
		)
	}

	if err := questionRepo.UpdateAnswer(
		ctx,
		stored.ID,
		"Yes",
		AnswerSourceCandidate,
		QuestionApproved,
	); err != nil {
		t.Fatal(err)
	}

	updated, err := questionRepo.GetByID(
		ctx,
		stored.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if updated == nil {
		t.Fatal("expected question")
	}

	if updated.Answer != "Yes" {
		t.Fatalf(
			"answer = %q, want Yes",
			updated.Answer,
		)
	}

	if updated.AnswerSource != AnswerSourceCandidate {
		t.Fatalf(
			"answer source = %q, want %q",
			updated.AnswerSource,
			AnswerSourceCandidate,
		)
	}

	if updated.Status != QuestionApproved {
		t.Fatalf(
			"status = %q, want %q",
			updated.Status,
			QuestionApproved,
		)
	}
}

func TestSQLiteQuestionRepositoryRejectsInvalidInput(
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

	repo := NewSQLiteQuestionRepository(db)
	ctx := context.Background()

	if err := repo.Create(ctx, ApplicationQuestion{}); err == nil {
		t.Fatal("expected Create to reject invalid application ID")
	}

	if _, err := repo.GetByID(ctx, 0); err == nil {
		t.Fatal("expected GetByID to reject invalid question ID")
	}

	if _, err := repo.ListByApplicationID(ctx, 0); err == nil {
		t.Fatal("expected ListByApplicationID to reject invalid application ID")
	}

	if err := repo.UpdateAnswer(
		ctx,
		0,
		"Yes",
		AnswerSourceCandidate,
		QuestionApproved,
	); err == nil {
		t.Fatal("expected UpdateAnswer to reject invalid question ID")
	}
}

func TestSQLiteQuestionRepositoryReturnsEmptyList(
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

	repo := NewSQLiteQuestionRepository(db)

	questions, err := repo.ListByApplicationID(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if questions == nil {
		t.Fatal("expected non-nil empty slice")
	}

	if len(questions) != 0 {
		t.Fatalf(
			"questions = %d, want 0",
			len(questions),
		)
	}
}
