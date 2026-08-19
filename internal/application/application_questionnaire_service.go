package application

import (
	"context"
	"fmt"
)

type ApplicationQuestionnaireService struct {
	extractor         QuestionnaireExtractor
	ingestor          *QuestionnaireIngestor
	questions         QuestionRepository
	resolver          *AnswerResolver
	questionAnswerLLM QuestionAnswerLLM
	candidateContext  string
	events            EventRepository
}

func NewApplicationQuestionnaireService(
	extractor QuestionnaireExtractor,
	ingestor *QuestionnaireIngestor,
	questions QuestionRepository,
	resolver *AnswerResolver,
	questionAnswerLLM QuestionAnswerLLM,
	candidateContext string,
	events EventRepository,
) *ApplicationQuestionnaireService {
	return &ApplicationQuestionnaireService{
		extractor:         extractor,
		ingestor:          ingestor,
		questions:         questions,
		resolver:          resolver,
		questionAnswerLLM: questionAnswerLLM,
		candidateContext:  candidateContext,
		events:            events,
	}
}

// ProcessSource extracts questionnaire questions, persists them, and resolves
// their answers using the existing QuestionnaireService pipeline.
func (s *ApplicationQuestionnaireService) ProcessSource(
	ctx context.Context,
	applicationID int64,
	raw string,
) ([]AnswerResolution, error) {
	if applicationID <= 0 {
		return nil, fmt.Errorf("application ID must be positive")
	}

	if s.extractor == nil {
		return nil, fmt.Errorf("questionnaire extractor is not configured")
	}

	if s.ingestor == nil {
		return nil, fmt.Errorf("questionnaire ingestor is not configured")
	}

	if s.questions == nil {
		return nil, fmt.Errorf("question repository is not configured")
	}

	if s.resolver == nil {
		return nil, fmt.Errorf("answer resolver is not configured")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	inputs, err := s.extractor.Extract(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("extract questionnaire: %w", err)
	}

	if err := s.ingestor.Ingest(
		ctx,
		applicationID,
		inputs,
	); err != nil {
		return nil, fmt.Errorf("ingest questionnaire: %w", err)
	}

	questionnaireService := NewQuestionnaireService(
		s.questions,
		s.resolver,
		s.questionAnswerLLM,
		s.candidateContext,
		s.events,
	)

	return questionnaireService.ProcessApplication(
		ctx,
		applicationID,
	)
}
