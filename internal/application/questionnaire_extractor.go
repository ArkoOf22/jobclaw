package application

import (
	"context"
	"fmt"
	"strings"
)

type QuestionnaireExtractor interface {
	Extract(
		ctx context.Context,
		raw string,
	) ([]QuestionnaireInput, error)
}

// TextQuestionnaireExtractor extracts questionnaire questions from
// line-oriented text. It performs no answer generation.
type TextQuestionnaireExtractor struct{}

func NewTextQuestionnaireExtractor() *TextQuestionnaireExtractor {
	return &TextQuestionnaireExtractor{}
}

func (e *TextQuestionnaireExtractor) Extract(
	ctx context.Context,
	raw string,
) ([]QuestionnaireInput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("questionnaire source is empty")
	}

	lines := strings.Split(raw, "\n")

	inputs := make([]QuestionnaireInput, 0)

	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = strings.TrimSpace(strings.TrimLeft(
			line,
			"-*•0123456789.)",
		))

		if line == "" {
			continue
		}

		inputs = append(inputs, QuestionnaireInput{
			Question: line,
		})
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("no questionnaire questions found")
	}

	return inputs, nil
}
