package application

import "strings"

type ResolutionStatus string

const (
	ResolutionAnswered    ResolutionStatus = "ANSWERED"
	ResolutionNeedsReview ResolutionStatus = "NEEDS_REVIEW"
)

type AnswerResolution struct {
	QuestionID int64
	FieldKey   string
	Answer     string
	Source     AnswerSource
	Status     ResolutionStatus
	Reason     string
}

func normalizeQuestion(question string) string {
	return strings.ToLower(strings.Join(strings.Fields(question), " "))
}

func classifyQuestionField(question string) string {
	q := normalizeQuestion(question)

	switch {
	case strings.Contains(q, "notice period"),
		strings.Contains(q, "serving notice"),
		strings.Contains(q, "how soon can you join"):
		return "notice_period"

	case strings.Contains(q, "work authorization"),
		strings.Contains(q, "authorized to work"),
		strings.Contains(q, "legally authorized"):
		return "work_authorization"

	case strings.Contains(q, "visa sponsorship"),
		strings.Contains(q, "require sponsorship"),
		strings.Contains(q, "sponsorship"):
		return "visa_sponsorship"

	case strings.Contains(q, "willing to relocate"),
		strings.Contains(q, "willingness to relocate"),
		strings.Contains(q, "relocate"):
		return "relocation"

	case strings.Contains(q, "years of experience"),
		strings.Contains(q, "years experience"),
		strings.Contains(q, "total experience"):
		return "years_experience"

	case strings.Contains(q, "salary expectation"),
		strings.Contains(q, "expected salary"),
		strings.Contains(q, "desired salary"),
		strings.Contains(q, "compensation expectation"):
		return "salary_expectation"

	default:
		return ""
	}
}
