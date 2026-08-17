package scoring

import (
	"testing"

	"jobclaw/internal/config"
)

func TestExtractMinimumYears(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
		ok   bool
	}{
		{
			name: "plain years",
			text: "2 years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "plus years",
			text: "2+ years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "minimum years",
			text: "minimum 3 years of experience",
			want: 3,
			ok:   true,
		},
		{
			name: "range",
			text: "2-4 years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "unicode range",
			text: "2–4 years experience",
			want: 2,
			ok:   true,
		},
		{
			name: "no experience requirement",
			text: "Build scalable backend services using Go and Kafka.",
			want: 0,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractMinimumYears(tt.text)

			if ok != tt.ok {
				t.Fatalf("found = %v, want %v", ok, tt.ok)
			}

			if got != tt.want {
				t.Fatalf("years = %.1f, want %.1f", got, tt.want)
			}
		})
	}
}

func TestScoreCandidateSkills(t *testing.T) {
	match := CandidateMatch{
		MatchedSkills:  0,
		RequiredSkills: 10,
	}

	if got := scoreCandidateSkills(match); got != 0 {
		t.Fatalf("score = %.1f, want 0.0", got)
	}

	match.MatchedSkills = 5

	if got := scoreCandidateSkills(match); got != 5.0 {
		t.Fatalf("score = %.1f, want 5.0", got)
	}

	match.MatchedSkills = 10

	if got := scoreCandidateSkills(match); got != 10.0 {
		t.Fatalf("score = %.1f, want 10.0", got)
	}
}

func TestScoreCandidateDomain(t *testing.T) {
	match := CandidateMatch{
		MatchedDomains:  0,
		RequiredDomains: 2,
	}

	if got := scoreCandidateDomain(match); got != 0 {
		t.Fatalf("score = %.1f, want 0.0", got)
	}

	match.MatchedDomains = 1

	if got := scoreCandidateDomain(match); got != 2.5 {
		t.Fatalf("score = %.1f, want 5.0", got)
	}

	match.MatchedDomains = 2

	if got := scoreCandidateDomain(match); got != 5 {
		t.Fatalf("score = %.1f, want 5.0", got)
	}
}

func TestScoreExperienceMissingRequirementIsFullScore(t *testing.T) {
	if got := scoreExperience(
		"Build scalable backend systems with Go and Kafka",
		2,
	); got != 15 {
		t.Fatalf("score = %.1f, want 15.0", got)
	}
}

func TestScoreExperienceThreeYearsForTwoYearCandidate(t *testing.T) {
	if got := scoreExperience(
		"3+ years of backend engineering experience",
		2,
	); got != 10 {
		t.Fatalf("score = %.1f, want 10.0", got)
	}
}

func TestScoreLocationUnknownGetsPartialScore(t *testing.T) {
	locations := config.Locations{
		Preferred:  []string{"Bangalore", "Bengaluru"},
		Acceptable: []string{"Remote", "Hyderabad"},
	}

	if got := scoreLocation("", locations); got != 5 {
		t.Fatalf("score = %.1f, want 5.0", got)
	}
}

func TestScoreCompensationMissingGetsNeutralScore(t *testing.T) {
	compensation := config.Compensation{
		TargetMinLPA: 21,
		TargetMaxLPA: 23,
	}

	if got := scoreCompensation(nil, nil, compensation); got != 5 {
		t.Fatalf("score = %.1f, want 5.0", got)
	}
}
