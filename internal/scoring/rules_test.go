package scoring

import (
	"testing"

	"jobclaw/internal/config"
)

func TestScoreCandidateSkills(t *testing.T) {
	match := CandidateMatch{
		MatchedSkills:  0,
		RequiredSkills: 10,
	}

	if got := scoreCandidateSkills(match); got != 0 {
		t.Fatalf("score = %.1f, want 0.0", got)
	}

	// Candidate-skill overlap is now the dominant signal, max 25 (was 10). A 50%
	// coverage of the posting's required skills therefore scores 12.5.
	match.MatchedSkills = 5

	if got := scoreCandidateSkills(match); got != 12.5 {
		t.Fatalf("score = %.1f, want 12.5", got)
	}

	match.MatchedSkills = 10

	if got := scoreCandidateSkills(match); got != 25.0 {
		t.Fatalf("score = %.1f, want 25.0", got)
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
	if got := scoreExperience(0, false, 2); got != 15 {
		t.Fatalf("score = %.1f, want 15.0", got)
	}
}

func TestScoreExperienceThreeYearsForTwoYearCandidate(t *testing.T) {
	if got := scoreExperience(3, true, 2); got != 10 {
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
