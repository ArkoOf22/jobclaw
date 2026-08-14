package company

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	repository *Repository
	collector  *EvidenceCollector
	classifier *EvidenceClassifier
}

func NewService(
	repository *Repository,
	collector *EvidenceCollector,
	classifier *EvidenceClassifier,
) *Service {
	return &Service{
		repository: repository,
		collector:  collector,
		classifier: classifier,
	}
}

func (s *Service) Classify(
	ctx context.Context,
	companyID int64,
) (*Company, error) {
	if companyID <= 0 {
		return nil, fmt.Errorf("company ID must be positive")
	}

	company, err := s.repository.GetByID(ctx, companyID)
	if err != nil {
		return nil, err
	}

	evidence, err := s.collector.Collect(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("collect company evidence: %w", err)
	}

	classification, reason := s.classifier.Classify(
		*company,
		evidence,
	)

	now := time.Now().UTC()

	if err := s.repository.UpdateClassification(
		ctx,
		companyID,
		classification,
		reason,
		now,
	); err != nil {
		return nil, err
	}

	company.Classification = classification
	company.ClassificationReason = reason
	company.ClassifiedAt = &now

	return company, nil
}
