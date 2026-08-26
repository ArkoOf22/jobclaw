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
		strings.Contains(q, "professional experience"),
		strings.Contains(q, "total experience"):
		return "years_experience"

	case strings.Contains(q, "salary expectation"),
		strings.Contains(q, "expected salary"),
		strings.Contains(q, "desired salary"),
		strings.Contains(q, "compensation expectation"):
		return "salary_expectation"

	case strings.Contains(q, "preferred programming language"),
		strings.Contains(q, "programming language"):
		return "preferred_programming_language"

	// Fields below are drawn from real Greenhouse forms, where custom questions
	// carry opaque IDs like "question_48620091" and can only be matched by text.

	case strings.Contains(q, "email"):
		return "email"

	case strings.Contains(q, "phone"),
		strings.Contains(q, "mobile number"):
		return "phone"

	// Checked before the broader employer and title cases, since "have you ever
	// been employed by" is about prior employment at this company, not the
	// candidate's current employer.
	case strings.Contains(q, "ever been employed by"),
		strings.Contains(q, "previously worked for"),
		strings.Contains(q, "ever worked for"),
		strings.Contains(q, "former employee"):
		return "previously_employed_here"

	case strings.Contains(q, "current or previous employer"),
		strings.Contains(q, "current employer"),
		strings.Contains(q, "most recent employer"):
		return "current_employer"

	case strings.Contains(q, "current or previous job title"),
		strings.Contains(q, "current job title"),
		strings.Contains(q, "current title"):
		return "current_title"

	// Remote intent is distinct from relocation, which is handled above.
	case strings.Contains(q, "work remotely"),
		strings.Contains(q, "plan to work remotely"),
		strings.Contains(q, "remote location"):
		return "remote_preference"

	case strings.Contains(q, "country where you currently reside"),
		strings.Contains(q, "country of residence"),
		strings.Contains(q, "where do you currently reside"):
		return "country_of_residence"

	case strings.Contains(q, "anticipate working in"),
		strings.Contains(q, "countries you anticipate"):
		return "work_countries"

	case strings.Contains(q, "city and state"),
		strings.Contains(q, "city and country"):
		return "city_and_state"

	case strings.Contains(q, "whatsapp"),
		strings.Contains(q, "opt-in to receive"),
		strings.Contains(q, "opt in to receive"),
		strings.Contains(q, "text messages"):
		return "messaging_opt_in"

	case strings.Contains(q, "first name"),
		strings.Contains(q, "given name"):
		return "first_name"

	case strings.Contains(q, "last name"),
		strings.Contains(q, "surname"),
		strings.Contains(q, "family name"):
		return "last_name"

	case strings.Contains(q, "linkedin"):
		return "linkedin_url"

	case strings.Contains(q, "github"):
		return "github_url"

	case strings.Contains(q, "website"),
		strings.Contains(q, "portfolio"):
		return "portfolio_url"

	default:
		return ""
	}
}
