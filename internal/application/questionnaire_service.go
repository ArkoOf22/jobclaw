package application

import (
	"context"
	"fmt"
)

type QuestionnaireService struct {
	questions QuestionRepository
	resolver  *AnswerResolver
	events    EventRepository
}

func NewQuestionnaireService(
	questions QuestionRepository,
	resolver *AnswerResolver,
	events EventRepository,
) *QuestionnaireService {
	return &QuestionnaireService{
		questions: questions,
		resolver:  resolver,
		events:    events,
	}
}

// ProcessApplication resolves every unanswered application question
// using only the verified candidate answer bank.
//
// Questions that cannot be safely answered remain NEEDS_REVIEW.
// The processor never invents or guesses an answer.
func (s *QuestionnaireService) ProcessApplication(
	ctx context.Context,
	applicationID int64,
) ([]AnswerResolution, error) {
	if applicationID <= 0 {
		return nil, fmt.Errorf("application ID must be positive")
	}

	if s.questions == nil {
		return nil, fmt.Errorf("question repository is not configured")
	}

	if s.resolver == nil {
		return nil, fmt.Errorf("answer resolver is not configured")
	}

	questions, err := s.questions.ListByApplicationID(
		ctx,
		applicationID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load application questions: %w",
			err,
		)
	}

	resolutions := make([]AnswerResolution, 0, len(questions))

	for _, question := range questions {
		// Do not overwrite questions that have already been approved.
		if question.Status == QuestionApproved {
			resolutions = append(resolutions, AnswerResolution{
				QuestionID: question.ID,
				FieldKey:   question.FieldKey,
				Answer:     question.Answer,
				Source:     question.AnswerSource,
				Status:     ResolutionAnswered,
				Reason:     "question is already approved",
			})
			continue
		}

		resolution, err := s.resolver.Resolve(
			ctx,
			question,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve question %d: %w",
				question.ID,
				err,
			)
		}

		if resolution.Status == ResolutionAnswered {
			if err := s.questions.UpdateAnswer(
				ctx,
				question.ID,
				resolution.Answer,
				resolution.Source,
				QuestionAnswered,
			); err != nil {
				return nil, fmt.Errorf(
					"persist answer for question %d: %w",
					question.ID,
					err,
				)
			}
		} else {
			// Explicitly persist NEEDS_REVIEW. This also makes the
			// state deterministic if the question previously had
			// an answer that is no longer considered safe.
			if err := s.questions.UpdateAnswer(
				ctx,
				question.ID,
				"",
				"",
				QuestionNeedsReview,
			); err != nil {
				return nil, fmt.Errorf(
					"mark question %d for review: %w",
					question.ID,
					err,
				)
			}
		}

		if s.events != nil {
			eventType := EventQuestionNeedsReview
			metadata := resolution.Reason

			if resolution.Status == ResolutionAnswered {
				eventType = EventQuestionAnswered
			}

			if err := s.events.Create(ctx, Event{
				ApplicationID: applicationID,
				Type:          eventType,
				Metadata:      metadata,
			}); err != nil {
				// Event logging is observational. A logging failure
				// must not invalidate an otherwise successful answer
				// resolution.
				_ = err
			}
		}

		resolutions = append(resolutions, resolution)
	}

	return resolutions, nil
}
