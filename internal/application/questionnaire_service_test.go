package application

import (
	"context"
	"fmt"
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
	if question.ID == 0 {
		question.ID = int64(len(f.questions) + 1)
	}

	f.questions = append(f.questions, question)

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

func (f *fakeQuestionRepository) FindByApplicationAndQuestion(
	ctx context.Context,
	applicationID int64,
	question string,
) (*ApplicationQuestion, error) {
	for _, q := range f.questions {
		if q.ApplicationID == applicationID &&
			q.Question == question {
			copy := q
			return &copy, nil
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
		nil,
		"",
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
		nil,
		"",
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
		nil,
		"",
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

type trackingQuestionAnswerLLM struct {
	answer    string
	err       error
	callCount int
}

func (f *trackingQuestionAnswerLLM) GenerateAnswer(
	ctx context.Context,
	question ApplicationQuestion,
	candidateContext string,
) (string, error) {
	f.callCount++

	if f.err != nil {
		return "", f.err
	}

	return f.answer, nil
}

func TestQuestionnaireServiceFallsBackToLLM(t *testing.T) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            40,
				ApplicationID: 40,
				Question:      "What is your preferred programming language?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	events := &fakeEventRepository{}

	llm := &trackingQuestionAnswerLLM{
		answer: "Go",
	}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate prefers Go for backend development.",
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		40,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	result := results[0]

	if result.Status != ResolutionAnswered {
		t.Fatalf(
			"status = %q, want %q",
			result.Status,
			ResolutionAnswered,
		)
	}

	if result.Answer != "Go" {
		t.Fatalf(
			"answer = %q, want %q",
			result.Answer,
			"Go",
		)
	}

	if result.Source != AnswerSourceLLM {
		t.Fatalf(
			"source = %q, want %q",
			result.Source,
			AnswerSourceLLM,
		)
	}

	if llm.callCount != 1 {
		t.Fatalf(
			"LLM calls = %d, want 1",
			llm.callCount,
		)
	}

	if len(questions.updates) != 1 {
		t.Fatalf(
			"updates = %d, want 1",
			len(questions.updates),
		)
	}

	if questions.updates[0].Answer != "Go" {
		t.Fatalf(
			"persisted answer = %q, want %q",
			questions.updates[0].Answer,
			"Go",
		)
	}

	if questions.updates[0].Source != AnswerSourceLLM {
		t.Fatalf(
			"persisted source = %q, want %q",
			questions.updates[0].Source,
			AnswerSourceLLM,
		)
	}

	if len(events.events) != 1 {
		t.Fatalf(
			"events = %d, want 1",
			len(events.events),
		)
	}

	if events.events[0].Type != EventQuestionAnswered {
		t.Fatalf(
			"event = %q, want %q",
			events.events[0].Type,
			EventQuestionAnswered,
		)
	}
}

func TestQuestionnaireServiceLLMNeedsReview(t *testing.T) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            41,
				ApplicationID: 41,
				Question:      "What is your preferred programming language?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	events := &fakeEventRepository{}

	llm := &trackingQuestionAnswerLLM{
		answer: "NEEDS_REVIEW",
	}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate profile contains no programming preference.",
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		41,
	)
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Status != ResolutionNeedsReview {
		t.Fatalf(
			"status = %q, want %q",
			results[0].Status,
			ResolutionNeedsReview,
		)
	}

	if results[0].Answer != "" {
		t.Fatalf(
			"answer = %q, want empty",
			results[0].Answer,
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
			"status = %q, want %q",
			questions.updates[0].Status,
			QuestionNeedsReview,
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

func TestQuestionnaireServiceCandidateAnswerWinsOverLLM(t *testing.T) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            42,
				ApplicationID: 42,
				Question:      "What is your notice period?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:        10,
				FieldKey:  "notice_period",
				Answer:    "60 days",
				Verified:  true,
				ValueType: AnswerValueText,
			},
		},
	}

	llm := &trackingQuestionAnswerLLM{
		answer: "30 days",
	}

	questionsEvents := &fakeEventRepository{}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate has a 30 day notice period.",
		questionsEvents,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		42,
	)
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Answer != "60 days" {
		t.Fatalf(
			"answer = %q, want %q",
			results[0].Answer,
			"60 days",
		)
	}

	if results[0].Source != AnswerSourceCandidate {
		t.Fatalf(
			"source = %q, want %q",
			results[0].Source,
			AnswerSourceCandidate,
		)
	}

	if llm.callCount != 0 {
		t.Fatalf(
			"LLM calls = %d, want 0",
			llm.callCount,
		)
	}
}

func TestQuestionnaireServiceLLMUnavailableLeavesQuestionForReview(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            43,
				ApplicationID: 43,
				Question:      "What is your preferred programming language?",
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
		nil,
		"",
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		43,
	)
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Status != ResolutionNeedsReview {
		t.Fatalf(
			"status = %q, want %q",
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
}

func TestQuestionnaireServiceContinuesAfterLLMFailure(t *testing.T) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            50,
				ApplicationID: 50,
				Question:      "What is your preferred programming language?",
				Status:        QuestionNeedsReview,
			},
			{
				ID:            51,
				ApplicationID: 50,
				Question:      "What is your notice period?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	llm := &trackingQuestionAnswerLLM{
		err: fmt.Errorf("OpenRouter timeout"),
	}

	events := &fakeEventRepository{}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate context",
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		50,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	for i, result := range results {
		if result.Status != ResolutionNeedsReview {
			t.Fatalf(
				"result[%d] status = %q, want %q",
				i,
				result.Status,
				ResolutionNeedsReview,
			)
		}

		if result.Answer != "" {
			t.Fatalf(
				"result[%d] answer = %q, want empty",
				i,
				result.Answer,
			)
		}
	}

	if len(questions.updates) != 2 {
		t.Fatalf(
			"updates = %d, want 2",
			len(questions.updates),
		)
	}

	for i, update := range questions.updates {
		if update.Status != QuestionNeedsReview {
			t.Fatalf(
				"update[%d] status = %q, want %q",
				i,
				update.Status,
				QuestionNeedsReview,
			)
		}
	}

	if len(events.events) != 2 {
		t.Fatalf(
			"events = %d, want 2",
			len(events.events),
		)
	}

	for i, event := range events.events {
		if event.Type != EventQuestionNeedsReview {
			t.Fatalf(
				"event[%d] type = %q, want %q",
				i,
				event.Type,
				EventQuestionNeedsReview,
			)
		}

		if event.Metadata == "" {
			t.Fatalf(
				"event[%d] metadata should contain failure reason",
				i,
			)
		}
	}

	if llm.callCount != 2 {
		t.Fatalf(
			"LLM calls = %d, want 2",
			llm.callCount,
		)
	}
}

func TestQuestionnaireServiceLLMFailureDoesNotAffectCandidateAnswers(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            52,
				ApplicationID: 52,
				Question:      "What is your notice period?",
				Status:        QuestionNeedsReview,
			},
			{
				ID:            53,
				ApplicationID: 52,
				Question:      "What is your preferred programming language?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:        20,
				FieldKey:  "notice_period",
				Answer:    "60 days",
				Verified:  true,
				ValueType: AnswerValueText,
			},
		},
	}

	llm := &trackingQuestionAnswerLLM{
		err: fmt.Errorf("OpenRouter unavailable"),
	}

	events := &fakeEventRepository{}

	service := NewQuestionnaireService(
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate context",
		events,
	)

	results, err := service.ProcessApplication(
		context.Background(),
		52,
	)
	if err != nil {
		t.Fatal(err)
	}

	if results[0].Status != ResolutionAnswered {
		t.Fatalf(
			"candidate answer status = %q, want %q",
			results[0].Status,
			ResolutionAnswered,
		)
	}

	if results[0].Answer != "60 days" {
		t.Fatalf(
			"candidate answer = %q, want %q",
			results[0].Answer,
			"60 days",
		)
	}

	if results[0].Source != AnswerSourceCandidate {
		t.Fatalf(
			"candidate source = %q, want %q",
			results[0].Source,
			AnswerSourceCandidate,
		)
	}

	if results[1].Status != ResolutionNeedsReview {
		t.Fatalf(
			"LLM question status = %q, want %q",
			results[1].Status,
			ResolutionNeedsReview,
		)
	}

	if llm.callCount != 1 {
		t.Fatalf(
			"LLM calls = %d, want 1",
			llm.callCount,
		)
	}
}
