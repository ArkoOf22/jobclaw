package application

import "time"

type Status string

const (
	StatusDraft                Status = "DRAFT"
	StatusReadyForReview       Status = "READY_FOR_REVIEW"
	StatusReadyToApply         Status = "READY_TO_APPLY"
	StatusSubmissionInProgress Status = "SUBMISSION_IN_PROGRESS"
	StatusApplied              Status = "APPLIED"
)

type Application struct {
	ID     int64
	JobID  int64
	Status Status

	TailoredResumePath     string
	CoverLetterPath        string
	ReferralMessagePath    string
	ApplicationAnswersPath string

	CreatedAt time.Time
	UpdatedAt time.Time
}
