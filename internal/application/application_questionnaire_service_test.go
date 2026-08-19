package application

import (
	"context"
	"errors"
	"testing"
)

var errTestQuestionnaireExtraction = errors.New(
	"test questionnaire extraction failure",
)

type fakeQuestionnaireExtractor struct {
	inputs []QuestionnaireInput
	err    error
	calls  int
}

func (f *fakeQuestionnaireExtractor) Extract(
	ctx context.Context,
	raw string,
) ([]QuestionnaireInput, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	return f.inputs, nil
}

func TestApplicationQuestionnaireServiceProcessesSource(t *testing.T) {
	questions := &fakeQuestionRepository{}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:        1,
				FieldKey:  "notice_period",
				Answer:    "60 days",
				ValueType: AnswerValueText,
				Verified:  true,
			},
		},
	}

	events := &fakeEventRepository{}

	extractor := &fakeQuestionnaireExtractor{
		inputs: []QuestionnaireInput{
			{
				Question: "What is your notice period?",
			},
		},
	}

	ingestor := NewQuestionnaireIngestor(
		questions,
		events,
	)

	service := NewApplicationQuestionnaireService(
		extractor,
		ingestor,
		questions,
		NewAnswerResolver(answers),
		nil,
		"",
		events,
	)

	results, err := service.ProcessSource(
		context.Background(),
		100,
		"ignored raw questionnaire",
	)
	if err != nil {
		t.Fatal(err)
	}

	if extractor.calls != 1 {
		t.Fatalf(
			"extractor calls = %d, want 1",
			extractor.calls,
		)
	}

	if len(questions.questions) != 1 {
		t.Fatalf(
			"questions = %d, want 1",
			len(questions.questions),
		)
	}

	if len(results) != 1 {
		t.Fatalf(
			"results = %d, want 1",
			len(results),
		)
	}

	if results[0].Status != ResolutionAnswered {
		t.Fatalf(
			"status = %q, want %q",
			results[0].Status,
			ResolutionAnswered,
		)
	}

	if results[0].Answer != "60 days" {
		t.Fatalf(
			"answer = %q, want %q",
			results[0].Answer,
			"60 days",
		)
	}
}

func TestApplicationQuestionnaireServicePropagatesExtractionFailure(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{}

	extractor := &fakeQuestionnaireExtractor{
		err: errTestQuestionnaireExtraction,
	}

	ingestor := NewQuestionnaireIngestor(
		questions,
		nil,
	)

	service := NewApplicationQuestionnaireService(
		extractor,
		ingestor,
		questions,
		NewAnswerResolver(
			&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{},
			},
		),
		nil,
		"",
		nil,
	)

	_, err := service.ProcessSource(
		context.Background(),
		100,
		"questionnaire",
	)
	if err == nil {
		t.Fatal("expected extraction error")
	}
}

func TestApplicationQuestionnaireServiceHonorsCancellation(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	extractor := &fakeQuestionnaireExtractor{
		inputs: []QuestionnaireInput{
			{
				Question: "What is your notice period?",
			},
		},
	}

	questions := &fakeQuestionRepository{}

	service := NewApplicationQuestionnaireService(
		extractor,
		NewQuestionnaireIngestor(questions, nil),
		questions,
		NewAnswerResolver(
			&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{},
			},
		),
		nil,
		"",
		nil,
	)

	_, err := service.ProcessSource(
		ctx,
		100,
		"questionnaire",
	)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

type integrationQuestionAnswerLLM struct {
	answer    string
	err       error
	callCount int
}

func (f *integrationQuestionAnswerLLM) GenerateAnswer(
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

func TestApplicationQuestionnaireServiceUsesLLMFallback(t *testing.T) {
	questions := &fakeQuestionRepository{}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	llm := &integrationQuestionAnswerLLM{
		answer: "Go",
	}

	extractor := &fakeQuestionnaireExtractor{
		inputs: []QuestionnaireInput{
			{
				Question: "What is your preferred programming language?",
			},
		},
	}

	events := &fakeEventRepository{}

	service := NewApplicationQuestionnaireService(
		extractor,
		NewQuestionnaireIngestor(questions, events),
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate prefers Go for backend development.",
		events,
	)

	results, err := service.ProcessSource(
		context.Background(),
		300,
		"What is your preferred programming language?",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Status != ResolutionAnswered {
		t.Fatalf(
			"status = %q, want %q",
			results[0].Status,
			ResolutionAnswered,
		)
	}

	if results[0].Answer != "Go" {
		t.Fatalf(
			"answer = %q, want %q",
			results[0].Answer,
			"Go",
		)
	}

	if results[0].Source != AnswerSourceLLM {
		t.Fatalf(
			"source = %q, want %q",
			results[0].Source,
			AnswerSourceLLM,
		)
	}

	if llm.callCount != 1 {
		t.Fatalf(
			"LLM calls = %d, want 1",
			llm.callCount,
		)
	}
}

func TestApplicationQuestionnaireServiceCandidateAnswerWinsOverLLM(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:        1,
				FieldKey:  "notice_period",
				Answer:    "60 days",
				ValueType: AnswerValueText,
				Verified:  true,
			},
		},
	}

	llm := &integrationQuestionAnswerLLM{
		answer: "30 days",
	}

	extractor := &fakeQuestionnaireExtractor{
		inputs: []QuestionnaireInput{
			{
				Question: "What is your notice period?",
			},
		},
	}

	events := &fakeEventRepository{}

	service := NewApplicationQuestionnaireService(
		extractor,
		NewQuestionnaireIngestor(questions, events),
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate context",
		events,
	)

	results, err := service.ProcessSource(
		context.Background(),
		301,
		"What is your notice period?",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
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

func TestApplicationQuestionnaireServiceLLMNeedsReview(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	llm := &integrationQuestionAnswerLLM{
		answer: "NEEDS_REVIEW",
	}

	extractor := &fakeQuestionnaireExtractor{
		inputs: []QuestionnaireInput{
			{
				Question: "What is your favorite programming language?",
			},
		},
	}

	events := &fakeEventRepository{}

	service := NewApplicationQuestionnaireService(
		extractor,
		NewQuestionnaireIngestor(questions, events),
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate context contains no preference.",
		events,
	)

	results, err := service.ProcessSource(
		context.Background(),
		302,
		"What is your favorite programming language?",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Status != ResolutionNeedsReview {
		t.Fatalf(
			"status = %q, want %q",
			results[0].Status,
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

func TestApplicationQuestionnaireServiceLLMFailureLeavesReview(
	t *testing.T,
) {
	questions := &fakeQuestionRepository{}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{},
	}

	llm := &integrationQuestionAnswerLLM{
		err: errTestQuestionnaireExtraction,
	}

	extractor := &fakeQuestionnaireExtractor{
		inputs: []QuestionnaireInput{
			{
				Question: "What is your favorite programming language?",
			},
		},
	}

	events := &fakeEventRepository{}

	service := NewApplicationQuestionnaireService(
		extractor,
		NewQuestionnaireIngestor(questions, events),
		questions,
		NewAnswerResolver(answers),
		llm,
		"Candidate context",
		events,
	)

	results, err := service.ProcessSource(
		context.Background(),
		303,
		"What is your favorite programming language?",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}

	if results[0].Status != ResolutionNeedsReview {
		t.Fatalf(
			"status = %q, want %q",
			results[0].Status,
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
