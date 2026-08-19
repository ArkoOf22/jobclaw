package application

import (
	"context"
	"fmt"
	"strings"
)

type ReadinessStatus string

const (
	ReadinessReady   ReadinessStatus = "READY"
	ReadinessBlocked ReadinessStatus = "BLOCKED"
)

type ReadinessCheck struct {
	Name   string
	Status ReadinessStatus
	Reason string
}

type ApplicationReadiness struct {
	ApplicationID int64
	Status        ReadinessStatus
	Checks        []ReadinessCheck
	Blockers      []string
}

func (r ApplicationReadiness) Ready() bool {
	return r.Status == ReadinessReady
}

type ApplicationReadinessEvaluator struct {
	applications Repository
	questions    QuestionRepository
}

func NewApplicationReadinessEvaluator(
	applications Repository,
	questions QuestionRepository,
) *ApplicationReadinessEvaluator {
	return &ApplicationReadinessEvaluator{
		applications: applications,
		questions:    questions,
	}
}

func (e *ApplicationReadinessEvaluator) Evaluate(
	ctx context.Context,
	applicationID int64,
) (ApplicationReadiness, error) {
	if applicationID <= 0 {
		return ApplicationReadiness{}, fmt.Errorf(
			"application ID must be positive",
		)
	}

	if e.applications == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"application repository is not configured",
		)
	}

	if e.questions == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"question repository is not configured",
		)
	}

	if err := ctx.Err(); err != nil {
		return ApplicationReadiness{}, err
	}

	app, err := e.applications.GetByID(ctx, applicationID)
	if err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"get application: %w",
			err,
		)
	}

	if app == nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"application %d not found",
			applicationID,
		)
	}

	readiness := ApplicationReadiness{
		ApplicationID: applicationID,
		Status:        ReadinessReady,
		Checks:        []ReadinessCheck{},
		Blockers:      []string{},
	}

	e.checkArtifact(
		&readiness,
		"tailored_resume",
		app.TailoredResumePath,
		"tailored resume is missing",
	)

	questions, err := e.questions.ListByApplicationID(
		ctx,
		applicationID,
	)
	if err != nil {
		return ApplicationReadiness{}, fmt.Errorf(
			"list application questions: %w",
			err,
		)
	}

	for _, question := range questions {
		if question.Status == QuestionNeedsReview {
			readiness.Status = ReadinessBlocked

			readiness.Blockers = append(
				readiness.Blockers,
				fmt.Sprintf(
					"question %d needs review: %s",
					question.ID,
					strings.TrimSpace(question.Question),
				),
			)
		}
	}

	if len(questions) == 0 {
		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   "questionnaire",
				Status: ReadinessReady,
				Reason: "no questionnaire questions",
			},
		)
	} else if readiness.Status == ReadinessReady {
		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   "questionnaire",
				Status: ReadinessReady,
				Reason: "all questionnaire questions are resolved",
			},
		)
	} else {
		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   "questionnaire",
				Status: ReadinessBlocked,
				Reason: "one or more questionnaire questions need review",
			},
		)
	}

	return readiness, nil
}

func (e *ApplicationReadinessEvaluator) checkArtifact(
	readiness *ApplicationReadiness,
	name string,
	path string,
	missingReason string,
) {
	if strings.TrimSpace(path) == "" {
		readiness.Status = ReadinessBlocked
		readiness.Blockers = append(
			readiness.Blockers,
			missingReason,
		)

		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   name,
				Status: ReadinessBlocked,
				Reason: missingReason,
			},
		)

		return
	}

	readiness.Checks = append(
		readiness.Checks,
		ReadinessCheck{
			Name:   name,
			Status: ReadinessReady,
			Reason: "artifact path is configured",
		},
	)
}
