package application

import (
	"context"
	"testing"
)

type fakeAnswerRepository struct {
	answers map[string]*CandidateAnswer
}

func (f *fakeAnswerRepository) GetByFieldKey(
	ctx context.Context,
	fieldKey string,
) (*CandidateAnswer, error) {
	return f.answers[fieldKey], nil
}

func TestAnswerResolverUsesVerifiedCandidateAnswer(t *testing.T) {
	repo := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:        1,
				FieldKey:  "notice_period",
				Answer:    "30 days",
				Verified:  true,
				ValueType: AnswerValueText,
			},
		},
	}

	resolver := NewAnswerResolver(repo)

	result, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       10,
			Question: "What is your notice period?",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != ResolutionAnswered {
		t.Fatalf("status = %q, want %q",
			result.Status, ResolutionAnswered)
	}

	if result.FieldKey != "notice_period" {
		t.Fatalf("field key = %q, want notice_period", result.FieldKey)
	}

	if result.Answer != "30 days" {
		t.Fatalf("answer = %q, want 30 days", result.Answer)
	}

	if result.Source != AnswerSourceCandidate {
		t.Fatalf("source = %q, want %q",
			result.Source, AnswerSourceCandidate)
	}
}

func TestAnswerResolverRejectsUnverifiedAnswer(t *testing.T) {
	repo := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"notice_period": {
				ID:       1,
				FieldKey: "notice_period",
				Answer:   "30 days",
				Verified: false,
			},
		},
	}

	resolver := NewAnswerResolver(repo)

	result, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       11,
			Question: "What is your notice period?",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != ResolutionNeedsReview {
		t.Fatalf("status = %q, want %q",
			result.Status, ResolutionNeedsReview)
	}
}

func TestAnswerResolverRequiresReviewForUnknownQuestion(t *testing.T) {
	resolver := NewAnswerResolver(
		&fakeAnswerRepository{
			answers: map[string]*CandidateAnswer{},
		},
	)

	result, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       12,
			Question: "Why do you want to work here?",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != ResolutionNeedsReview {
		t.Fatalf("status = %q, want %q",
			result.Status, ResolutionNeedsReview)
	}

	if result.Answer != "" {
		t.Fatalf("answer = %q, want empty", result.Answer)
	}
}

func TestAnswerResolverRejectsEmptyAnswer(t *testing.T) {
	repo := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"work_authorization": {
				ID:       2,
				FieldKey: "work_authorization",
				Answer:   "   ",
				Verified: true,
			},
		},
	}

	resolver := NewAnswerResolver(repo)

	result, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       13,
			Question: "Are you legally authorized to work?",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != ResolutionNeedsReview {
		t.Fatalf("status = %q, want %q",
			result.Status, ResolutionNeedsReview)
	}
}

func TestClassifyQuestionFieldRealisticVariants(t *testing.T) {
	tests := []struct {
		name     string
		question string
		want     string
	}{
		{
			name:     "notice period",
			question: "What is your notice period?",
			want:     "notice_period",
		},
		{
			name:     "joining time",
			question: "How soon can you join?",
			want:     "notice_period",
		},
		{
			name:     "work authorization",
			question: "Are you legally authorized to work in India?",
			want:     "work_authorization",
		},
		{
			name:     "visa sponsorship",
			question: "Will you require visa sponsorship?",
			want:     "visa_sponsorship",
		},
		{
			name:     "relocation",
			question: "Are you willing to relocate?",
			want:     "relocation",
		},
		{
			name:     "years experience",
			question: "How many years of professional experience do you have?",
			want:     "years_experience",
		},
		{
			name:     "salary",
			question: "What are your salary expectations?",
			want:     "salary_expectation",
		},
		{
			name:     "unknown motivation",
			question: "Why do you want to work here?",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyQuestionField(tt.question)

			if got != tt.want {
				t.Fatalf(
					"classifyQuestionField(%q) = %q, want %q",
					tt.question,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestAnswerResolverClassifiesQuestionBeforeLLMFallback(t *testing.T) {
	repo := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"years_experience": {
				ID:        20,
				FieldKey:  "years_experience",
				Answer:    "2 years",
				Verified:  true,
				ValueType: AnswerValueText,
			},
		},
	}

	resolver := NewAnswerResolver(repo)

	result, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       21,
			Question: "How many years of professional experience do you have?",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != ResolutionAnswered {
		t.Fatalf(
			"status = %q, want %q",
			result.Status,
			ResolutionAnswered,
		)
	}

	if result.FieldKey != "years_experience" {
		t.Fatalf(
			"field key = %q, want %q",
			result.FieldKey,
			"years_experience",
		)
	}

	if result.Answer != "2 years" {
		t.Fatalf(
			"answer = %q, want %q",
			result.Answer,
			"2 years",
		)
	}

	if result.Source != AnswerSourceCandidate {
		t.Fatalf(
			"source = %q, want %q",
			result.Source,
			AnswerSourceCandidate,
		)
	}
}
