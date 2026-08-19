package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type QuestionnaireInput struct {
	Question string
	FieldKey string
	Metadata string
}

type QuestionnaireIngestor struct {
	questions QuestionRepository
	events    EventRepository
}

func NewQuestionnaireIngestor(
	questions QuestionRepository,
	events EventRepository,
) *QuestionnaireIngestor {
	return &QuestionnaireIngestor{
		questions: questions,
		events:    events,
	}
}

// Ingest persists raw application questions without attempting to answer them.
//
// Questions are deliberately created as NEEDS_REVIEW. Answer resolution is a
// separate concern handled by QuestionnaireService.
func (i *QuestionnaireIngestor) Ingest(
	ctx context.Context,
	applicationID int64,
	inputs []QuestionnaireInput,
) error {
	if applicationID <= 0 {
		return fmt.Errorf("application ID must be positive")
	}

	if i.questions == nil {
		return fmt.Errorf("question repository is not configured")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	for index, input := range inputs {
		question := strings.TrimSpace(input.Question)

		if question == "" {
			return fmt.Errorf(
				"question at index %d is empty",
				index,
			)
		}

		fieldKey := strings.TrimSpace(input.FieldKey)
		metadata := strings.TrimSpace(input.Metadata)

		err := i.questions.Create(
			ctx,
			ApplicationQuestion{
				ApplicationID: applicationID,
				Question:      question,
				FieldKey:      fieldKey,
				Metadata:      metadata,
				Status:        QuestionNeedsReview,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"create questionnaire question %d: %w",
				index,
				err,
			)
		}
	}

	if i.events != nil {
		if err := i.events.Create(
			ctx,
			Event{
				ApplicationID: applicationID,
				Type:          EventQuestionnaireIngested,
				Metadata:      strconv.Itoa(len(inputs)),
			},
		); err != nil {
			// Event logging is observational. Successful question
			// ingestion must not be rolled back because event logging
			// failed.
			_ = err
		}
	}

	return nil
}
