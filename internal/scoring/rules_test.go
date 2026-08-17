package scoring

import "testing"

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
