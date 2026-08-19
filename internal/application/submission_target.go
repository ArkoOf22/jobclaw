package application

import (
	"context"

	"jobclaw/internal/job"
)

type SubmissionTargetType string

const (
	SubmissionTargetManual     SubmissionTargetType = "MANUAL"
	SubmissionTargetGreenhouse SubmissionTargetType = "GREENHOUSE"
	SubmissionTargetLever      SubmissionTargetType = "LEVER"
	SubmissionTargetWorkday    SubmissionTargetType = "WORKDAY"
	SubmissionTargetGeneric    SubmissionTargetType = "GENERIC"
)

type SubmissionTarget struct {
	Type SubmissionTargetType
	URL  string

	// ExternalID identifies the job/application target in the external
	// system when one is available.
	ExternalID string
}

type SubmissionRequest struct {
	Application Application
	Job         job.Job
	Target      SubmissionTarget
	Prepared    PreparedSubmission
}

type SubmissionAdapter interface {
	Submit(
		ctx context.Context,
		request SubmissionRequest,
	) (SubmissionResult, error)
}

type SubmissionAdapterRegistry interface {
	Get(targetType SubmissionTargetType) (SubmissionAdapter, error)
}

func ResolveSubmissionTarget(j job.Job) SubmissionTarget {
	targetType := SubmissionTargetGeneric

	switch j.Source {
	case "greenhouse", "Greenhouse":
		targetType = SubmissionTargetGreenhouse
	case "lever", "Lever":
		targetType = SubmissionTargetLever
	case "workday", "Workday":
		targetType = SubmissionTargetWorkday
	}

	return SubmissionTarget{
		Type:       targetType,
		URL:        j.URL,
		ExternalID: j.ExternalID,
	}
}
