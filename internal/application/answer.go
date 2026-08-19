package application

import "time"

type AnswerValueType string

const (
	AnswerValueText    AnswerValueType = "TEXT"
	AnswerValueBoolean AnswerValueType = "BOOLEAN"
	AnswerValueNumber  AnswerValueType = "NUMBER"
)

type CandidateAnswer struct {
	ID int64

	FieldKey string
	Question string

	Answer    string
	ValueType AnswerValueType
	Verified  bool
	Notes     string

	CreatedAt time.Time
	UpdatedAt time.Time
}
