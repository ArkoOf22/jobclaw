package application

import (
	"context"
	"encoding/json"
	"fmt"
)

type ApplicationPreparationService struct {
	applications  Repository
	events        EventRepository
	questionnaire *QuestionnaireService
	readiness     *ApplicationReadinessEvaluator
}

func NewApplicationPreparationService(
	applications Repository,
	questions QuestionRepository,
	events EventRepository,
	questionnaire *QuestionnaireService,
) *ApplicationPreparationService {
	return &ApplicationPreparationService{
		applications:  applications,
		events:        events,
		questionnaire: questionnaire,
		readiness: NewApplicationReadinessEvaluator(
			applications,
			questions,
		),
	}
}

func (s *ApplicationPreparationService) Prepare(
	ctx context.Context,
	applicationID int64,
) (ApplicationReadiness, error) {
	if applicationID <= 0 {
		return ApplicationReadiness{}, fmt.Errorf(
			"application ID must be positive",
		)
	}

	if s.applications == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"application repository is not configured",
		)
	}

	if s.events == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"event repository is not configured",
		)
	}

	if s.questionnaire == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"questionnaire service is not configured",
		)
	}

	if _, err := s.questionnaire.ProcessApplication(
		ctx,
		applicationID,
	); err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"process application questionnaire: %w",
			err,
		)
	}

	readiness, err := s.readiness.Evaluate(
		ctx,
		applicationID,
	)
	if err != nil {
		return ApplicationReadiness{}, err
	}

	if !readiness.Ready() {
		return readiness, nil
	}

	app, err := s.applications.GetByID(
		ctx,
		applicationID,
	)
	if err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"get application before preparation: %w",
			err,
		)
	}

	if app == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"application %d not found",
			applicationID,
		)
	}

	if app.Status == StatusReadyToApply {
		return readiness, nil
	}

	if app.Status != StatusDraft {
		return ApplicationReadiness{}, fmt.Errorf(
			"application %d cannot be prepared from status %q",
			applicationID,
			app.Status,
		)
	}

	if err := s.applications.UpdateStatus(
		ctx,
		applicationID,
		StatusReadyToApply,
	); err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"mark application ready to apply: %w",
			err,
		)
	}

	metadata, err := json.Marshal(map[string]any{
		"application_id": applicationID,
		"status":         StatusReadyToApply,
	})
	if err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"build application ready event metadata: %w",
			err,
		)
	}

	if err := s.events.Create(
		ctx,
		Event{
			ApplicationID: applicationID,
			Type:          EventApplicationReady,
			Metadata:      string(metadata),
		},
	); err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"record application ready event: %w",
			err,
		)
	}

	return readiness, nil
}
