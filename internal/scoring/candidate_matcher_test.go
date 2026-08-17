package scoring

import (
	"testing"

	"jobclaw/internal/config"
)

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func TestCandidateMatcherMatchesSkills(t *testing.T) {
	candidate := config.Candidate{
		Skills: config.Skills{
			Languages: []string{
				"Go",
				"Java",
			},
			Backend: []string{
				"Kafka",
				"Redis",
				"Microservices",
				"Concurrency",
			},
			Databases: []string{
				"PostgreSQL",
			},
			CloudInfrastructure: []string{
				"AWS",
				"Docker",
			},
		},
		DomainExperience: []string{
			"Fintech",
			"Payments",
			"Distributed Systems",
		},
	}

	matcher := NewCandidateMatcher(candidate)

	result := matcher.Match(`
		Build distributed backend services using Go and Kafka.
		Experience with PostgreSQL, Redis, Docker and AWS.
		Work on fintech payment infrastructure.
	`, testPreferences())

	if result.MatchedSkills == 0 {
		t.Fatal("expected candidate skill matches")
	}

	if result.MatchedDomains == 0 {
		t.Fatal("expected candidate domain matches")
	}

	if !containsString(result.Skills, "Go") {
		t.Fatal("expected Go to be matched")
	}

	if !containsString(result.Skills, "Kafka") {
		t.Fatal("expected Kafka to be matched")
	}

	if !containsString(result.Skills, "PostgreSQL") {
		t.Fatal("expected PostgreSQL to be matched")
	}

	if !containsString(result.Domains, "Fintech") {
		t.Fatal("expected Fintech to be matched")
	}

	if !containsString(result.Domains, "Payments") {
		t.Fatal("expected Payments to be matched")
	}
}

func TestCandidateMatcherDoesNotMatchMissingSkills(t *testing.T) {
	candidate := config.Candidate{
		Skills: config.Skills{
			Languages: []string{"Go"},
			Backend:   []string{"Kafka"},
		},
	}

	matcher := NewCandidateMatcher(candidate)

	result := matcher.Match(`
		Build services using Go and Kafka.
		Experience with Rust and Cassandra.
	`, testPreferences())

	if !containsString(result.Skills, "Go") {
		t.Fatal("expected Go to be matched")
	}

	if !containsString(result.Skills, "Kafka") {
		t.Fatal("expected Kafka to be matched")
	}

	if containsString(result.Skills, "Rust") {
		t.Fatal("Rust should not be reported as candidate skill")
	}

	if containsString(result.Skills, "Cassandra") {
		t.Fatal("Cassandra should not be reported as candidate skill")
	}
}

func TestCandidateMatcherIsCaseInsensitive(t *testing.T) {
	candidate := config.Candidate{
		Skills: config.Skills{
			Languages: []string{"Go"},
			Backend:   []string{"Kafka"},
		},
	}

	matcher := NewCandidateMatcher(candidate)

	result := matcher.Match(`
		GO backend services with KAFKA.
	`, testPreferences())

	if result.MatchedSkills != 2 {
		t.Fatalf(
			"matched skills = %d, want 2",
			result.MatchedSkills,
		)
	}
}

func TestCandidateMatcherRatios(t *testing.T) {
	candidate := config.Candidate{
		Skills: config.Skills{
			Languages: []string{"Go", "Java"},
			Backend:   []string{"Kafka", "Redis"},
		},
		DomainExperience: []string{
			"Fintech",
			"Payments",
		},
	}

	matcher := NewCandidateMatcher(candidate)

	result := matcher.Match(`
		Backend services using Go and Kafka
		for fintech payments infrastructure.
	`, testPreferences())

	if result.RequiredSkills != 3 {
		t.Fatalf(
			"required skills = %d, want 3",
			result.RequiredSkills,
		)
	}

	if result.MatchedSkills != 2 {
		t.Fatalf(
			"matched skills = %d, want 2",
			result.MatchedSkills,
		)
	}

	if result.SkillMatchRatio() != 2.0/3.0 {
		t.Fatalf(
			"skill match ratio = %.2f, want 0.67",
			result.SkillMatchRatio(),
		)
	}

	if result.RequiredDomains != 2 {
		t.Fatalf(
			"required domains = %d, want 2",
			result.RequiredDomains,
		)
	}

	if result.MatchedDomains != 2 {
		t.Fatalf(
			"matched domains = %d, want 2",
			result.MatchedDomains,
		)
	}

	if result.DomainMatchRatio() != 1.0 {
		t.Fatalf(
			"domain match ratio = %.2f, want 1.00",
			result.DomainMatchRatio(),
		)
	}
}

func TestCandidateMatcherDoesNotMatchGoInsideGoogle(t *testing.T) {
	candidate := config.Candidate{
		Skills: config.Skills{
			Languages: []string{"Go"},
		},
	}

	matcher := NewCandidateMatcher(candidate)

	result := matcher.Match(
		"Experience working with Google Cloud Platform.",
		testPreferences(),
	)

	if result.MatchedSkills != 0 {
		t.Fatalf(
			"matched skills = %d, want 0",
			result.MatchedSkills,
		)
	}
}

func TestContainsTermHandlesSingularPlural(t *testing.T) {
	tests := []struct {
		text string
		term string
	}{
		{
			text: "payment infrastructure",
			term: "Payments",
		},
		{
			text: "distributed systems",
			term: "Distributed Systems",
		},
		{
			text: "microservices platform",
			term: "Microservices",
		},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			if !containsTerm(tt.text, tt.term) {
				t.Fatalf(
					"containsTerm(%q, %q) = false, want true",
					tt.text,
					tt.term,
				)
			}
		})
	}
}

func TestContainsTermDoesNotMatchSubstring(t *testing.T) {
	if containsTerm(
		"Experience working with Google Cloud Platform",
		"Go",
	) {
		t.Fatal("Go should not match inside Google")
	}
}
