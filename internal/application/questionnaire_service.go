package application

import (
	"context"
	"fmt"
	"strings"
)

type QuestionnaireService struct {
	questions         QuestionRepository
	resolver          *AnswerResolver
	questionAnswerLLM QuestionAnswerLLM
	candidateContext  string
	events            EventRepository
}

func NewQuestionnaireService(
	questions QuestionRepository,
	resolver *AnswerResolver,
	questionAnswerLLM QuestionAnswerLLM,
	candidateContext string,
	events EventRepository,
) *QuestionnaireService {
	return &QuestionnaireService{
		questions:         questions,
		resolver:          resolver,
		questionAnswerLLM: questionAnswerLLM,
		candidateContext:  candidateContext,
		events:            events,
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

		// Leave questions that already carry an answer alone.
		//
		// Re-resolving them is destructive: the resolver only knows the verified
		// answer bank, so an answer produced by any other source resolves to
		// NEEDS_REVIEW and gets wiped. Preparation re-runs this pipeline, which
		// meant every `prepare` erased the answers the preceding
		// `questionnaire` run had produced. Skipping them also keeps this
		// idempotent and avoids repeat LLM calls on every invocation.
		//
		// Answers that are not candidate-verified are surfaced by the
		// answer_provenance readiness check and in the submission dry run, so
		// they still get human review before anything is sent.
		if question.Status == QuestionAnswered &&
			strings.TrimSpace(question.Answer) != "" {
			resolutions = append(resolutions, AnswerResolution{
				QuestionID: question.ID,
				FieldKey:   question.FieldKey,
				Answer:     question.Answer,
				Source:     question.AnswerSource,
				Status:     ResolutionAnswered,
				Reason:     "question already has an answer",
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

		// Candidate answer bank is authoritative. Only fall back
		// to the LLM when the verified answer bank cannot answer.
		if resolution.Status == ResolutionNeedsReview &&
			s.questionAnswerLLM != nil &&
			s.candidateContext != "" {

			answer, llmErr := s.questionAnswerLLM.GenerateAnswer(
				ctx,
				question,
				s.candidateContext,
			)

			if llmErr != nil {
				// LLM failures are isolated to this question.
				// The questionnaire must continue processing the
				// remaining questions.
				resolution.Reason = fmt.Sprintf(
					"LLM answer generation failed: %v",
					llmErr,
				)
			} else if answer != "NEEDS_REVIEW" {
				resolution.Answer = answer
				resolution.Source = AnswerSourceLLM
				resolution.Status = ResolutionAnswered
				resolution.Reason = "answered from candidate context using LLM"
			} else {
				resolution.Reason =
					"LLM could not safely answer from candidate context"
			}
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
