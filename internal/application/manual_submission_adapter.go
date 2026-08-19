package application

import (
	"context"
	"fmt"
)

type ManualSubmissionAdapter struct{}

func NewManualSubmissionAdapter() *ManualSubmissionAdapter {
	return &ManualSubmissionAdapter{}
}

func (a *ManualSubmissionAdapter) Submit(
	ctx context.Context,
	request SubmissionRequest,
) (SubmissionResult, error) {
	if err := ctx.Err(); err != nil {
		return SubmissionFailed, err
	}

	if request.Application.ID <= 0 {
		return SubmissionFailed, fmt.Errorf(
			"application ID must be positive",
		)
	}

	if request.Job.ID <= 0 {
		return SubmissionFailed, fmt.Errorf(
			"job ID must be positive",
		)
	}

	return SubmissionFailed, fmt.Errorf(
		"manual submission required for application %d (job %d)",
		request.Application.ID,
		request.Job.ID,
	)
}
