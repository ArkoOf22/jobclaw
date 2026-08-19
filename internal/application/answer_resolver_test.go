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
