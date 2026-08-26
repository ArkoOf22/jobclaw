package application

import (
	"context"
	"fmt"
	"os"
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

	events EventRepository
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

// WithEventRepository lets readiness distinguish "the form genuinely has no
// questions" from "questionnaire ingestion never ran".
//
// Without it, zero questions is indistinguishable from never having looked, and
// an application can be declared READY and submitted with every field blank.
// Optional so existing callers keep working; when unset, readiness reports the
// ambiguity rather than assuming either way.
func (e *ApplicationReadinessEvaluator) WithEventRepository(
	events EventRepository,
) *ApplicationReadinessEvaluator {
	e.events = events

	return e
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

	questionnaireBlocked := false

	for _, question := range questions {
		label := strings.TrimSpace(question.Question)

		if question.Status == QuestionNeedsReview {
			questionnaireBlocked = true

			readiness.Status = ReadinessBlocked

			readiness.Blockers = append(
				readiness.Blockers,
				fmt.Sprintf(
					"question %d needs review: %s",
					question.ID,
					label,
				),
			)

			continue
		}

		// A question marked ANSWERED or APPROVED with an empty answer would
		// otherwise pass readiness and be submitted blank. Status alone is not
		// evidence that an answer exists.
		if strings.TrimSpace(question.Answer) == "" {
			questionnaireBlocked = true

			readiness.Status = ReadinessBlocked

			readiness.Blockers = append(
				readiness.Blockers,
				fmt.Sprintf(
					"question %d is marked %s but has no answer: %s",
					question.ID,
					question.Status,
					label,
				),
			)
		}
	}

	e.checkQuestionnaireCoverage(
		ctx,
		&readiness,
		applicationID,
		questions,
		questionnaireBlocked,
	)

	e.reportUnverifiedAnswers(&readiness, questions)

	return readiness, nil
}

// checkQuestionnaireCoverage records whether the questionnaire can be trusted as
// complete. Zero questions is only acceptable evidence when ingestion actually
// ran and found nothing.
func (e *ApplicationReadinessEvaluator) checkQuestionnaireCoverage(
	ctx context.Context,
	readiness *ApplicationReadiness,
	applicationID int64,
	questions []ApplicationQuestion,
	blocked bool,
) {
	if blocked {
		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   "questionnaire",
				Status: ReadinessBlocked,
				Reason: "one or more questionnaire questions are unresolved",
			},
		)

		return
	}

	if len(questions) > 0 {
		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   "questionnaire",
				Status: ReadinessReady,
				Reason: fmt.Sprintf(
					"all %d questionnaire question(s) are resolved",
					len(questions),
				),
			},
		)

		return
	}

	ingested, err := e.questionnaireWasIngested(ctx, applicationID)

	if err != nil || !ingested {
		reason := "questionnaire has not been ingested, so the form is unverified"

		if err != nil {
			reason = fmt.Sprintf(
				"cannot confirm questionnaire ingestion: %v",
				err,
			)
		}

		readiness.Status = ReadinessBlocked
		readiness.Blockers = append(readiness.Blockers, reason)

		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   "questionnaire",
				Status: ReadinessBlocked,
				Reason: reason,
			},
		)

		return
	}

	readiness.Checks = append(
		readiness.Checks,
		ReadinessCheck{
			Name:   "questionnaire",
			Status: ReadinessReady,
			Reason: "questionnaire was ingested and the form has no questions",
		},
	)
}

func (e *ApplicationReadinessEvaluator) questionnaireWasIngested(
	ctx context.Context,
	applicationID int64,
) (bool, error) {
	if e.events == nil {
		return false, fmt.Errorf(
			"event repository is not configured",
		)
	}

	events, err := e.events.ListByApplicationID(ctx, applicationID)
	if err != nil {
		return false, err
	}

	for _, event := range events {
		if event.Type == EventQuestionnaireIngested {
			return true, nil
		}
	}

	return false, nil
}

// reportUnverifiedAnswers surfaces LLM-generated answers without blocking.
//
// An LLM answer is not verified candidate information, and the project's stated
// policy is to surface risky answers rather than let them through silently. This
// is advisory on purpose: blocking here would stall the workflow, so the
// decision stays with the operator reviewing the dry-run payload.
func (e *ApplicationReadinessEvaluator) reportUnverifiedAnswers(
	readiness *ApplicationReadiness,
	questions []ApplicationQuestion,
) {
	var unverified []string

	for _, question := range questions {
		if question.AnswerSource != AnswerSourceLLM {
			continue
		}

		if question.Status == QuestionApproved {
			continue
		}

		unverified = append(
			unverified,
			strings.TrimSpace(question.Question),
		)
	}

	if len(unverified) == 0 {
		return
	}

	readiness.Checks = append(
		readiness.Checks,
		ReadinessCheck{
			Name:   "answer_provenance",
			Status: ReadinessReady,
			Reason: fmt.Sprintf(
				"%d answer(s) are LLM-generated and not candidate-verified: %s",
				len(unverified),
				strings.Join(unverified, "; "),
			),
		},
	)
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

	// A recorded path is not evidence that the artifact exists. Without this,
	// readiness passes for a resume that was deleted, truncated, or never
	// actually written, and submission proceeds with nothing to attach.
	info, err := os.Stat(path)

	if err != nil {
		reason := fmt.Sprintf(
			"artifact path is recorded but unreadable: %s",
			path,
		)

		if os.IsNotExist(err) {
			reason = fmt.Sprintf(
				"artifact path is recorded but the file does not exist: %s",
				path,
			)
		}

		readiness.Status = ReadinessBlocked
		readiness.Blockers = append(readiness.Blockers, reason)

		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   name,
				Status: ReadinessBlocked,
				Reason: reason,
			},
		)

		return
	}

	if info.Size() == 0 {
		reason := fmt.Sprintf(
			"artifact exists but is empty: %s",
			path,
		)

		readiness.Status = ReadinessBlocked
		readiness.Blockers = append(readiness.Blockers, reason)

		readiness.Checks = append(
			readiness.Checks,
			ReadinessCheck{
				Name:   name,
				Status: ReadinessBlocked,
				Reason: reason,
			},
		)

		return
	}

	readiness.Checks = append(
		readiness.Checks,
		ReadinessCheck{
			Name:   name,
			Status: ReadinessReady,
			Reason: fmt.Sprintf(
				"artifact present (%d bytes)",
				info.Size(),
			),
		},
	)
}
