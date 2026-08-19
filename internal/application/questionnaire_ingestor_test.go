package application

import (
	"context"
	"errors"
	"testing"
)

func TestQuestionnaireIngestorCreatesQuestions(t *testing.T) {
	questions := &fakeQuestionRepository{}
	events := &fakeEventRepository{}

	ingestor := NewQuestionnaireIngestor(
		questions,
		events,
	)

	err := ingestor.Ingest(
		context.Background(),
		100,
		[]QuestionnaireInput{
			{
				Question: "  What is your notice period?  ",
				FieldKey: " notice_period ",
				Metadata: " source=ats ",
			},
			{
				Question: "Are you willing to relocate?",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(events.events) != 1 {
		t.Fatalf("events = %d, want 1", len(events.events))
	}

	if events.events[0].Type != EventQuestionnaireIngested {
		t.Fatalf(
			"event type = %q, want %q",
			events.events[0].Type,
			EventQuestionnaireIngested,
		)
	}

	if events.events[0].Metadata != "2" {
		t.Fatalf(
			"event metadata = %q, want %q",
			events.events[0].Metadata,
			"2",
		)
	}
}

func TestQuestionnaireIngestorRejectsEmptyQuestion(t *testing.T) {
	questions := &fakeQuestionRepository{}
	events := &fakeEventRepository{}

	ingestor := NewQuestionnaireIngestor(
		questions,
		events,
	)

	err := ingestor.Ingest(
		context.Background(),
		100,
		[]QuestionnaireInput{
			{
				Question: "What is your notice period?",
			},
			{
				Question: "   ",
			},
		},
	)
	if err == nil {
		t.Fatal("expected empty question error")
	}

	if len(events.events) != 0 {
		t.Fatalf(
			"events = %d, want 0",
			len(events.events),
		)
	}
}

func TestQuestionnaireIngestorRejectsInvalidApplication(t *testing.T) {
	ingestor := NewQuestionnaireIngestor(
		&fakeQuestionRepository{},
		&fakeEventRepository{},
	)

	err := ingestor.Ingest(
		context.Background(),
		0,
		nil,
	)

	if err == nil {
		t.Fatal("expected invalid application ID error")
	}
}

func TestQuestionnaireIngestorRequiresRepository(t *testing.T) {
	ingestor := NewQuestionnaireIngestor(
		nil,
		&fakeEventRepository{},
	)

	err := ingestor.Ingest(
		context.Background(),
		100,
		nil,
	)

	if err == nil {
		t.Fatal("expected repository configuration error")
	}
}

func TestQuestionnaireIngestorHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ingestor := NewQuestionnaireIngestor(
		&fakeQuestionRepository{},
		&fakeEventRepository{},
	)

	err := ingestor.Ingest(
		ctx,
		100,
		nil,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"error = %v, want context.Canceled",
			err,
		)
	}
}

func TestQuestionnaireIngestorPersistsNormalizedQuestions(t *testing.T) {
	questions := &fakeQuestionRepository{}
	events := &fakeEventRepository{}

	ingestor := NewQuestionnaireIngestor(
		questions,
		events,
	)

	err := ingestor.Ingest(
		context.Background(),
		200,
		[]QuestionnaireInput{
			{
				Question: "  What is your notice period?  ",
				FieldKey: " notice_period ",
				Metadata: " source=ats ",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(questions.questions) != 1 {
		t.Fatalf(
			"questions = %d, want 1",
			len(questions.questions),
		)
	}

	question := questions.questions[0]

	if question.ApplicationID != 200 {
		t.Fatalf(
			"application ID = %d, want 200",
			question.ApplicationID,
		)
	}

	if question.Question != "What is your notice period?" {
		t.Fatalf(
			"question = %q, want %q",
			question.Question,
			"What is your notice period?",
		)
	}

	if question.FieldKey != "notice_period" {
		t.Fatalf(
			"field key = %q, want %q",
			question.FieldKey,
			"notice_period",
		)
	}

	if question.Metadata != "source=ats" {
		t.Fatalf(
			"metadata = %q, want %q",
			question.Metadata,
			"source=ats",
		)
	}

	if question.Status != QuestionNeedsReview {
		t.Fatalf(
			"status = %q, want %q",
			question.Status,
			QuestionNeedsReview,
		)
	}

	if question.Answer != "" {
		t.Fatalf(
			"answer = %q, want empty",
			question.Answer,
		)
	}

	if question.AnswerSource != "" {
		t.Fatalf(
			"answer source = %q, want empty",
			question.AnswerSource,
		)
	}
}

func TestQuestionnaireIngestorIsIdempotent(t *testing.T) {
	questions := &fakeQuestionRepository{}
	events := &fakeEventRepository{}

	ingestor := NewQuestionnaireIngestor(
		questions,
		events,
	)

	inputs := []QuestionnaireInput{
		{
			Question: "What is your notice period?",
			FieldKey: "notice_period",
		},
		{
			Question: "Are you willing to relocate?",
			FieldKey: "relocation",
		},
	}

	if err := ingestor.Ingest(
		context.Background(),
		300,
		inputs,
	); err != nil {
		t.Fatal(err)
	}

	if err := ingestor.Ingest(
		context.Background(),
		300,
		inputs,
	); err != nil {
		t.Fatal(err)
	}

	if len(questions.questions) != 2 {
		t.Fatalf(
			"questions = %d, want 2",
			len(questions.questions),
		)
	}
}

func TestQuestionnaireIngestorDoesNotOverwriteExistingAnswer(t *testing.T) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            10,
				ApplicationID: 301,
				Question:      "What is your notice period?",
				FieldKey:      "notice_period",
				Answer:        "60 days",
				AnswerSource:  AnswerSourceManual,
				Status:        QuestionApproved,
			},
		},
	}

	ingestor := NewQuestionnaireIngestor(
		questions,
		nil,
	)

	err := ingestor.Ingest(
		context.Background(),
		301,
		[]QuestionnaireInput{
			{
				Question: "What is your notice period?",
				FieldKey: "notice_period",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(questions.questions) != 1 {
		t.Fatalf(
			"questions = %d, want 1",
			len(questions.questions),
		)
	}

	q := questions.questions[0]

	if q.Answer != "60 days" {
		t.Fatalf(
			"answer = %q, want %q",
			q.Answer,
			"60 days",
		)
	}

	if q.Status != QuestionApproved {
		t.Fatalf(
			"status = %q, want %q",
			q.Status,
			QuestionApproved,
		)
	}
}
