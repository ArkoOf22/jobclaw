package application

import (
	"context"
	"testing"
)

type fakeQuestionRepository struct {
	questions []ApplicationQuestion
	updates   []questionUpdate
}

type questionUpdate struct {
	ID     int64
	Answer string
	Source AnswerSource
	Status QuestionStatus
}

func (f *fakeQuestionRepository) Create(
	ctx context.Context,
	question ApplicationQuestion,
) error {
	return nil
}

func (f *fakeQuestionRepository) GetByID(
	ctx context.Context,
	id int64,
) (*ApplicationQuestion, error) {
	for _, question := range f.questions {
		if question.ID == id {
			q := question
			return &q, nil
		}
	}

	return nil, nil
}

func (f *fakeQuestionRepository) ListByApplicationID(
	ctx context.Context,
	applicationID int64,
) ([]ApplicationQuestion, error) {
	return f.questions, nil
}

func (f *fakeQuestionRepository) UpdateAnswer(
	ctx context.Context,
	id int64,
	answer string,
	source AnswerSource,
	status QuestionStatus,
) error {
	f.updates = append(f.updates, questionUpdate{
		ID:     id,
		Answer: answer,
		Source: source,
		Status: status,
	})

	return nil
}

type fakeEventRepository struct {
	events []Event
}

func (f *fakeEventRepository) Create(
	ctx context.Context,
	event Event,
) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeEventRepository) ListByApplicationID(
	ctx context.Context,
	applicationID int64,
) ([]Event, error) {
	return f.events, nil
}

func TestQuestionnaireServiceResolvesVerifiedAnswers(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            1,
				ApplicationID: 10,
				Question:      "What is your notice period?",
				Status:        QuestionNeedsReview,
			},
			{
				ID:            2,
				ApplicationID: 10,
				Question:      "Are you willing to relocate?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:        1,
				FieldKey:  "notice_period",
				Answer:    "30 days",
				ValueType: AnswerValueText,
				Verified:  true,
			},
			"relocation": {
				ID:        2,
				FieldKey:  "relocation",
				Answer:    "Yes",
				ValueType: AnswerValueText,
				Verified:  true,
			},
		},
	}

	events := &fakeEventRepository{}

	resolver := NewAnswerResolver(answers)

	service := NewQuestionnaireService(
		questions,
		resolver,
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		10,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	if len(questions.updates) != 2 {
		t.Fatalf(
			"updates = %d, want 2",
			len(questions.updates),
		)
	}

	if questions.updates[0].Answer != "30 days" {
		t.Fatalf(
			"first answer = %q, want %q",
			questions.updates[0].Answer,
			"30 days",
		)
	}

	if questions.updates[0].Status != QuestionAnswered {
		t.Fatalf(
			"first status = %q, want %q",
			questions.updates[0].Status,
			QuestionAnswered,
		)
	}

	if questions.updates[1].Answer != "Yes" {
		t.Fatalf(
			"second answer = %q, want %q",
			questions.updates[1].Answer,
			"Yes",
		)
	}

	if questions.updates[1].Status != QuestionAnswered {
		t.Fatalf(
			"second status = %q, want %q",
			questions.updates[1].Status,
			QuestionAnswered,
		)
	}

	if len(events.events) != 2 {
		t.Fatalf(
			"events = %d, want 2",
			len(events.events),
		)
	}

	if events.events[0].Type != EventQuestionAnswered {
		t.Fatalf(
			"first event = %q, want %q",
			events.events[0].Type,
			EventQuestionAnswered,
		)
	}
}

func TestQuestionnaireServiceMarksUnknownQuestionsForReview(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            5,
				ApplicationID: 20,
				Question:      "What is your favorite programming language?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	events := &fakeEventRepository{}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		20,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Status != ResolutionNeedsReview {
		t.Fatalf(
			"resolution status = %q, want %q",
			results[0].Status,
			ResolutionNeedsReview,
		)
	}

	if len(questions.updates) != 1 {
		t.Fatalf(
			"updates = %d, want 1",
			len(questions.updates),
		)
	}

	if questions.updates[0].Status != QuestionNeedsReview {
		t.Fatalf(
			"question status = %q, want %q",
			questions.updates[0].Status,
			QuestionNeedsReview,
		)
	}

	if questions.updates[0].Answer != "" {
		t.Fatalf(
			"answer = %q, want empty",
			questions.updates[0].Answer,
		)
	}

	if len(events.events) != 1 {
		t.Fatalf(
			"events = %d, want 1",
			len(events.events),
		)
	}

	if events.events[0].Type != EventQuestionNeedsReview {
		t.Fatalf(
			"event = %q, want %q",
			events.events[0].Type,
			EventQuestionNeedsReview,
		)
	}
}

func TestQuestionnaireServiceDoesNotOverwriteApprovedQuestion(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            9,
				ApplicationID: 30,
				Question:      "What is your notice period?",
				FieldKey:      "notice_period",
				Answer:        "45 days",
				AnswerSource:  AnswerSourceManual,
				Status:        QuestionApproved,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:       1,
				FieldKey: "notice_period",
				Answer:   "30 days",
				Verified: true,
			},
		},
	}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		&fakeEventRepository{},
	)

	results, err := service.ProcessApplication(
		context.Background(),
		30,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Answer != "45 days" {
		t.Fatalf(
			"answer = %q, want %q",
			results[0].Answer,
			"45 days",
		)
	}

	if len(questions.updates) != 0 {
		t.Fatalf(
			"updates = %d, want 0",
			len(questions.updates),
		)
	}
}
