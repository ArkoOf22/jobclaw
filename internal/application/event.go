package application

import "time"

type EventType string

const (
	EventApplicationCreated           EventType = "APPLICATION_CREATED"
	EventResumeGenerationStarted      EventType = "RESUME_GENERATION_STARTED"
	EventResumeGenerationFailed       EventType = "RESUME_GENERATION_FAILED"
	EventResumeGenerationSucceeded    EventType = "RESUME_GENERATION_SUCCEEDED"
	EventResumeValidationFailed       EventType = "RESUME_VALIDATION_FAILED"
	EventQuestionnaireIngested        EventType = "QUESTIONNAIRE_INGESTED"
	EventQuestionAnswered             EventType = "QUESTION_ANSWERED"
	EventQuestionNeedsReview          EventType = "QUESTION_NEEDS_REVIEW"
	EventApplicationReady             EventType = "APPLICATION_READY"
	EventApplicationSubmissionStarted EventType = "APPLICATION_SUBMISSION_STARTED"
	EventApplicationSubmitted         EventType = "APPLICATION_SUBMITTED"
	EventApplicationSubmissionFailed  EventType = "APPLICATION_SUBMISSION_FAILED"
)

type Event struct {
	ID            int64
	ApplicationID int64
	Type          EventType
	Metadata      string
	CreatedAt     time.Time
}
