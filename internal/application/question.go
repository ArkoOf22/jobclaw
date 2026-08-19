package application

import "time"

type QuestionStatus string

const (
	QuestionNeedsReview QuestionStatus = "NEEDS_REVIEW"
	QuestionAnswered    QuestionStatus = "ANSWERED"
	QuestionApproved    QuestionStatus = "APPROVED"
)

type AnswerSource string

const (
	AnswerSourceCandidate    AnswerSource = "CANDIDATE"
	AnswerSourceMasterResume AnswerSource = "MASTER_RESUME"
	AnswerSourceLLM          AnswerSource = "LLM"
	AnswerSourceManual       AnswerSource = "MANUAL"
)

type ApplicationQuestion struct {
	ID            int64
	ApplicationID int64

	Question string
	FieldKey string

	Answer       string
	AnswerSource AnswerSource
	Status       QuestionStatus

	Metadata string

	CreatedAt time.Time
	UpdatedAt time.Time
}
