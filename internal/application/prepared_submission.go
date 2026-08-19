package application

import (
	"context"
	"fmt"
	"os"
	"strings"

	"jobclaw/internal/job"
)

type ResolvedSubmissionAnswer struct {
	QuestionID int64
	FieldKey   string
	Question   string
	Answer     string
}

type PreparedSubmission struct {
	Application Application
	Job         job.Job
	Resume      []byte
	Answers     []ResolvedSubmissionAnswer
}

type SubmissionDataProvider struct {
	questions QuestionRepository
	resolver  *AnswerResolver
}

func NewSubmissionDataProvider(
	questions QuestionRepository,
	resolver *AnswerResolver,
) *SubmissionDataProvider {
	return &SubmissionDataProvider{
		questions: questions,
		resolver:  resolver,
	}
}

func (p *SubmissionDataProvider) Prepare(
	ctx context.Context,
	app Application,
	j job.Job,
) (PreparedSubmission, error) {
	if app.ID <= 0 {
		return PreparedSubmission{}, fmt.Errorf(
			"application ID must be positive",
		)
	}

	if j.ID <= 0 {
		return PreparedSubmission{}, fmt.Errorf(
			"job ID must be positive",
		)
	}

	if p.questions == nil {
		return PreparedSubmission{}, fmt.Errorf(
			"question repository is not configured",
		)
	}

	if p.resolver == nil {
		return PreparedSubmission{}, fmt.Errorf(
			"answer resolver is not configured",
		)
	}

	if err := ctx.Err(); err != nil {
		return PreparedSubmission{}, err
	}

	resumePath := strings.TrimSpace(app.TailoredResumePath)
	if resumePath == "" {
		return PreparedSubmission{}, fmt.Errorf(
			"tailored resume is missing",
		)
	}

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		return PreparedSubmission{}, fmt.Errorf(
			"read tailored resume: %w",
			err,
		)
	}

	if len(resume) == 0 {
		return PreparedSubmission{}, fmt.Errorf(
			"tailored resume is empty",
		)
	}

	questions, err := p.questions.ListByApplicationID(
		ctx,
		app.ID,
	)
	if err != nil {
		return PreparedSubmission{}, fmt.Errorf(
			"list application questions: %w",
			err,
		)
	}

	answers := make(
		[]ResolvedSubmissionAnswer,
		0,
		len(questions),
	)

	for _, question := range questions {
		resolution, err := p.resolver.Resolve(
			ctx,
			question,
		)
		if err != nil {
			return PreparedSubmission{}, fmt.Errorf(
				"resolve question %d: %w",
				question.ID,
				err,
			)
		}

		if resolution.Status != ResolutionAnswered {
			return PreparedSubmission{}, fmt.Errorf(
				"question %d cannot be submitted: %s",
				question.ID,
				resolution.Reason,
			)
		}

		answers = append(
			answers,
			ResolvedSubmissionAnswer{
				QuestionID: question.ID,
				FieldKey:   resolution.FieldKey,
				Question:   question.Question,
				Answer:     resolution.Answer,
			},
		)
	}

	return PreparedSubmission{
		Application: app,
		Job:         j,
		Resume:      resume,
		Answers:     answers,
	}, nil
}
