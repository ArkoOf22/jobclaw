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

func TestClassifyQuestionFieldPreferredProgrammingLanguage(t *testing.T) {
	tests := []string{
		"What is your preferred programming language?",
		"Which programming language do you prefer?",
		"What programming language are you most comfortable with?",
	}

	for _, question := range tests {
		if got := classifyQuestionField(question); got != "preferred_programming_language" {
			t.Fatalf(
				"classifyQuestionField(%q) = %q, want preferred_programming_language",
				question,
				got,
			)
		}
	}
}

// ATS platforms name custom questions opaquely. Greenhouse uses IDs like
// "question_48620091" for "Are you authorized to work...", so matching only on
// the literal field key would tie a verified answer to one employer's one form.
func TestAnswerResolverFallsBackToSemanticFieldKey(t *testing.T) {
	testCases := []struct {
		name         string
		question     string
		opaqueKey    string
		bankKey      string
		wantFieldKey string
	}{
		{
			name:         "work authorization",
			question:     "Are you authorized to work in the location(s) you selected?",
			opaqueKey:    "question_48620091",
			bankKey:      "work_authorization",
			wantFieldKey: "work_authorization",
		},
		{
			name:         "visa sponsorship",
			question:     "Will you require Stripe to sponsor you for a work permit?",
			opaqueKey:    "question_48620092",
			bankKey:      "visa_sponsorship",
			wantFieldKey: "visa_sponsorship",
		},
		{
			name:         "remote intent is not relocation",
			question:     "If this role offers the option to work from a remote location, do you plan to work remotely?",
			opaqueKey:    "question_48620093",
			bankKey:      "remote_preference",
			wantFieldKey: "remote_preference",
		},
		{
			name:         "prior employment at this company",
			question:     "Have you ever been employed by Stripe or a Stripe affiliate?",
			opaqueKey:    "question_48620094",
			bankKey:      "previously_employed_here",
			wantFieldKey: "previously_employed_here",
		},
		{
			name:         "country of residence",
			question:     "Please select the country where you currently reside.",
			opaqueKey:    "question_48620089",
			bankKey:      "country_of_residence",
			wantFieldKey: "country_of_residence",
		},
		{
			name:         "messaging opt-in",
			question:     "Do you opt-in to receive WhatsApp messages from Stripe Recruiting?",
			opaqueKey:    "question_49691714",
			bankKey:      "messaging_opt_in",
			wantFieldKey: "messaging_opt_in",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolver := NewAnswerResolver(&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{
					testCase.bankKey: {
						FieldKey: testCase.bankKey,
						Answer:   "Yes",
						Verified: true,
					},
				},
			})

			resolution, err := resolver.Resolve(
				context.Background(),
				ApplicationQuestion{
					ID:       1,
					Question: testCase.question,
					FieldKey: testCase.opaqueKey,
				},
			)
			if err != nil {
				t.Fatal(err)
			}

			if resolution.Status != ResolutionAnswered {
				t.Fatalf(
					"status = %q, want %q; reason = %s",
					resolution.Status,
					ResolutionAnswered,
					resolution.Reason,
				)
			}

			if resolution.FieldKey != testCase.wantFieldKey {
				t.Fatalf(
					"field key = %q, want %q",
					resolution.FieldKey,
					testCase.wantFieldKey,
				)
			}

			if resolution.Source != AnswerSourceCandidate {
				t.Fatalf(
					"source = %q, want %q",
					resolution.Source,
					AnswerSourceCandidate,
				)
			}
		})
	}
}

// The literal field key must win when it has an answer, so a deliberately
// employer-specific answer is not shadowed by a generic one.
func TestAnswerResolverPrefersLiteralFieldKey(t *testing.T) {
	resolver := NewAnswerResolver(&fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"question_48620091": {
				FieldKey: "question_48620091",
				Answer:   "Employer-specific answer",
				Verified: true,
			},
			"work_authorization": {
				FieldKey: "work_authorization",
				Answer:   "Generic answer",
				Verified: true,
			},
		},
	})

	resolution, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       1,
			Question: "Are you authorized to work in India?",
			FieldKey: "question_48620091",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if resolution.Answer != "Employer-specific answer" {
		t.Fatalf(
			"answer = %q, want the literal field key to take precedence",
			resolution.Answer,
		)
	}
}

// An unverified answer must not be used, even via the semantic fallback.
func TestAnswerResolverIgnoresUnverifiedSemanticMatch(t *testing.T) {
	resolver := NewAnswerResolver(&fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"work_authorization": {
				FieldKey: "work_authorization",
				Answer:   "Yes",
				Verified: false,
			},
		},
	})

	resolution, err := resolver.Resolve(
		context.Background(),
		ApplicationQuestion{
			ID:       1,
			Question: "Are you legally authorized to work?",
			FieldKey: "question_1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if resolution.Status != ResolutionNeedsReview {
		t.Fatalf(
			"status = %q, want %q for an unverified answer",
			resolution.Status,
			ResolutionNeedsReview,
		)
	}
}
