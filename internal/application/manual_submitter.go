package application

import (
	"context"
	"fmt"

	"jobclaw/internal/job"
)

type ManualSubmitter struct{}

func NewManualSubmitter() *ManualSubmitter {
	return &ManualSubmitter{}
}

func (s *ManualSubmitter) Submit(
	ctx context.Context,
	app Application,
	j job.Job,
) (SubmissionResult, error) {
	if err := ctx.Err(); err != nil {
		return SubmissionFailed, err
	}

	if app.ID <= 0 {
		return SubmissionFailed, fmt.Errorf(
			"application ID must be positive",
		)
	}

	if j.ID <= 0 {
		return SubmissionFailed, fmt.Errorf(
			"job ID must be positive",
		)
	}

	return SubmissionFailed, fmt.Errorf(
		"external application submission is not configured for application %d",
		app.ID,
	)
}
