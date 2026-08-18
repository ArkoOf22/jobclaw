package application

import "time"

type EventType string

const (
	EventApplicationCreated        EventType = "APPLICATION_CREATED"
	EventResumeGenerationStarted   EventType = "RESUME_GENERATION_STARTED"
	EventResumeGenerationFailed    EventType = "RESUME_GENERATION_FAILED"
	EventResumeGenerationSucceeded EventType = "RESUME_GENERATION_SUCCEEDED"
	EventResumeValidationFailed    EventType = "RESUME_VALIDATION_FAILED"
)

type Event struct {
	ID            int64
	ApplicationID int64
	Type          EventType
	Metadata      string
	CreatedAt     time.Time
}
