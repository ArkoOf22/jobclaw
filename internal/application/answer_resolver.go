package application

import (
	"context"
	"fmt"
	"strings"
)

type CandidateAnswerLookup interface {
	GetByFieldKey(
		ctx context.Context,
		fieldKey string,
	) (*CandidateAnswer, error)
}

type AnswerResolver struct {
	answers CandidateAnswerLookup
}

func NewAnswerResolver(answers CandidateAnswerLookup) *AnswerResolver {
	return &AnswerResolver{
		answers: answers,
	}
}

func (r *AnswerResolver) Resolve(
	ctx context.Context,
	question ApplicationQuestion,
) (AnswerResolution, error) {
	if question.ID <= 0 {
		return AnswerResolution{}, fmt.Errorf("question ID must be positive")
	}

	if strings.TrimSpace(question.Question) == "" {
		return AnswerResolution{}, fmt.Errorf("question is required")
	}

	fieldKey := strings.TrimSpace(question.FieldKey)

	if fieldKey == "" {
		fieldKey = classifyQuestionField(question.Question)
	}

	if fieldKey == "" {
		return AnswerResolution{
			QuestionID: question.ID,
			Status:     ResolutionNeedsReview,
			Reason:     "question could not be mapped to a known candidate field",
		}, nil
	}

	if r.answers == nil {
		return AnswerResolution{
			QuestionID: question.ID,
			FieldKey:   fieldKey,
			Status:     ResolutionNeedsReview,
			Reason:     "candidate answer repository is not configured",
		}, nil
	}

	answer, err := r.answers.GetByFieldKey(ctx, fieldKey)
	if err != nil {
		return AnswerResolution{}, fmt.Errorf("lookup candidate answer: %w", err)
	}

	if answer == nil {
		return AnswerResolution{
			QuestionID: question.ID,
			FieldKey:   fieldKey,
			Status:     ResolutionNeedsReview,
			Reason:     "no candidate answer exists for this field",
		}, nil
	}

	if !answer.Verified {
		return AnswerResolution{
			QuestionID: question.ID,
			FieldKey:   fieldKey,
			Status:     ResolutionNeedsReview,
			Reason:     "candidate answer is not verified",
		}, nil
	}

	if strings.TrimSpace(answer.Answer) == "" {
		return AnswerResolution{
			QuestionID: question.ID,
			FieldKey:   fieldKey,
			Status:     ResolutionNeedsReview,
			Reason:     "candidate answer is empty",
		}, nil
	}

	return AnswerResolution{
		QuestionID: question.ID,
		FieldKey:   fieldKey,
		Answer:     answer.Answer,
		Source:     AnswerSourceCandidate,
		Status:     ResolutionAnswered,
		Reason:     "matched verified candidate answer",
	}, nil
}
