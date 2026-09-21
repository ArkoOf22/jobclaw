package scoring

import (
	"strings"
	"testing"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/job"
)

func TestExtractRequiredYears(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		want        float64
		ok          bool
	}{
		{
			name:        "plain years",
			description: "2 years experience",
			want:        2,
			ok:          true,
		},
		{
			name:        "plus years",
			description: "2+ years of experience",
			want:        2,
			ok:          true,
		},
		{
			name:        "minimum years",
			description: "minimum 3 years of experience",
			want:        3,
			ok:          true,
		},
		{
			name:        "range takes the low end",
			description: "2-4 years experience",
			want:        2,
			ok:          true,
		},
		{
			name:        "en dash range",
			description: "2\u20134 years of professional experience",
			want:        2,
			ok:          true,
		},
		{
			name:        "to range",
			description: "Job Requirements: 1 to 2 years of software development",
			want:        1,
			ok:          true,
		},
		{
			name:        "decimal years",
			description: "Experience : 1.00 + years",
			want:        1,
			ok:          true,
		},
		{
			name:        "year with parenthesised plural",
			description: "Minimum 3 Year(s) Of Experience Is Required",
			want:        3,
			ok:          true,
		},
		{
			name:        "no requirement stated",
			description: "Build scalable backend services using Go and Kafka.",
			want:        0,
			ok:          false,
		},

		// Regression: descriptions are stored as escaped HTML. Splitting on
		// whitespace left "&lt;li&gt;8+" beside "years", which parsed as no
		// requirement at all, so the posting scored full marks on experience.
		{
			name: "escaped html list item",
			description: "&lt;p&gt;Minimum requirements&lt;/p&gt;" +
				"&lt;ul&gt;&lt;li&gt;8+ years of experience designing systems&lt;/li&gt;&lt;/ul&gt;",
			want: 8,
			ok:   true,
		},
		{
			name:        "escaped html non breaking space",
			description: "&lt;li&gt;5-7&amp;nbsp;years experience writing production code&lt;/li&gt;",
			want:        5,
			ok:          true,
		},

		// Regression: only the literal word "year" was recognised, so the
		// abbreviated form common in Naukri titles read as no requirement.
		{
			name:  "abbreviated years in title",
			title: "Java Developer (Rest API, Microservices)_4+Yrs_Bangalore/Pune",
			want:  4,
			ok:    true,
		},
		{
			name:  "years in title need no supporting language",
			title: "Software Engineer, React Native (3-5 Years)",
			want:  3,
			ok:    true,
		},

		// Regression: only the first mention was read. Every stated bar has to
		// be met, so the binding one is the highest.
		{
			name: "highest of several stated bars",
			description: "Must have: 3-5 years of hands-on backend experience. " +
				"Preferred: 8+ years of experience with distributed systems.",
			want: 8,
			ok:   true,
		},

		// A figure with no requirement language near it is prose, not a bar.
		// Vetoing on it would silently hide a job the candidate wanted.
		{
			name:        "incidental mention is ignored",
			description: "Founded 12 years ago, we have grown 10x in 3 years.",
			want:        0,
			ok:          false,
		},
		{
			name: "schooling years are not experience",
			description: "Educational Qualification: 15 years full time education. " +
				"Relevant experience: 2 years.",
			want: 2,
			ok:   true,
		},

		// Regression: these two lines run together in the source posting, and a
		// shared exclusion window let "Educational" on the second line suppress
		// the real requirement on the first.
		{
			name: "requirement immediately followed by schooling line",
			description: "Minimum 3 Year(s) Of Experience Is Required " +
				"Educational Qualification : 15 years full time education",
			want: 3,
			ok:   true,
		},
		{
			name:        "schooling line alone states no requirement",
			description: "Educational Qualification : Min 15 years of full time education",
			want:        0,
			ok:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractRequiredYears(
				tt.title,
				job.NormalizeDescription(tt.description),
			)

			if ok != tt.ok {
				t.Fatalf("found = %v, want %v", ok, tt.ok)
			}

			if got != tt.want {
				t.Fatalf("years = %.1f, want %.1f", got, tt.want)
			}
		})
	}
}

// twoYearScorer mirrors the live configuration: a two-year candidate with a
// ceiling of exactly two years and no stretch.
func twoYearScorer() *Scorer {
	preferences := testPreferences()
	preferences.ExperienceRequired.MaxRequiredYears = 2

	return NewScorer(
		config.Candidate{
			Experience: config.Experience{
				TotalYears: 2,
			},
			Skills: config.Skills{
				Languages: []string{"Go", "Java"},
				Backend: []string{
					"Kafka",
					"Redis",
					"Microservices",
					"Concurrency",
				},
				Databases: []string{"PostgreSQL"},
			},
			DomainExperience: []string{
				"SaaS",
				"Distributed Systems",
				"Payments",
			},
		},
		preferences,
	)
}

// The posting this reproduces scored 77.3 and reached the shortlist, with
// experience=15.0, purely because the escaped markup hid its "8+ years".
func TestScorerVetoesEscapedHTMLSeniorRequirement(t *testing.T) {
	result := twoYearScorer().Score(
		job.Job{
			Title: "Software Engineer, Internal Systems",
			Description: "&lt;p&gt;Build backend services in Go with Kafka, Redis and " +
				"concurrency for a payments platform.&lt;/p&gt;" +
				"&lt;p&gt;Minimum requirements&lt;/p&gt;" +
				"&lt;ul&gt;&lt;li&gt;8+ years of experience designing distributed systems&lt;/li&gt;&lt;/ul&gt;",
			Location: "Bengaluru, Karnataka",
		},
		company.Company{
			Classification: company.ClassificationProduct,
		},
	)

	if result.Recommendation != RecommendationSkip {
		t.Fatalf(
			"recommendation = %s, want SKIP: score=%.1f reasoning=%s",
			result.Recommendation,
			result.OverallScore,
			result.Reasoning,
		)
	}

	if result.ExperienceScore != 0 {
		t.Fatalf("experience score = %.1f, want 0", result.ExperienceScore)
	}
}

// "3-5 years" must be vetoed for a two-year candidate.
//
// This test previously asserted the opposite. The ceiling was set above the
// candidate's years so a slightly-senior role would stay visible with a reduced
// score, on the theory that the reader could weigh the gap themselves. Measured
// against a real shortlist that was wrong: 3-5, 5-6 and 3-8 year roles crowded
// the list and the candidate spent their time opening links for jobs they could
// not apply to. The low end of this range is 3, above a two-year candidate, so
// it goes.
func TestScorerVetoesThreeToFiveYearRequirement(t *testing.T) {
	result := twoYearScorer().Score(
		job.Job{
			Title: "Backend Engineer",
			Description: "Build backend services in Go with Kafka, Redis and concurrency " +
				"for a SaaS distributed systems platform. " +
				"Key Skills And Experience: 3-5 years of software engineering.",
			Location: "Bengaluru",
		},
		company.Company{
			Classification: company.ClassificationProduct,
		},
	)

	if result.Recommendation != RecommendationSkip {
		t.Fatalf(
			"recommendation = %s, want SKIP: score=%.1f reasoning=%s",
			result.Recommendation,
			result.OverallScore,
			result.Reasoning,
		)
	}

	// The figure read must be the low end of the range, and it must be recorded,
	// so a veto can be audited without re-parsing the description.
	if !strings.Contains(result.Reasoning, "required_years=3") {
		t.Errorf("reasoning does not record required_years=3: %s", result.Reasoning)
	}

	if !strings.Contains(result.Reasoning, "ceiling_years=2.0") {
		t.Errorf("reasoning does not record ceiling_years=2.0: %s", result.Reasoning)
	}
}

// A range whose low end the candidate meets is a genuine match and must survive,
// otherwise tightening the ceiling would hide the jobs worth seeing.
func TestScorerKeepsRangeStartingAtCandidateYears(t *testing.T) {
	result := twoYearScorer().Score(
		job.Job{
			Title: "Backend Engineer",
			Description: "&lt;p&gt;We are looking for a Backend Engineer with 2-4 years of " +
				"professional experience in Go, Kafka, Redis, PostgreSQL and concurrency " +
				"on a SaaS payments platform built on distributed systems.&lt;/p&gt;",
			Location: "Bengaluru, Karnataka",
		},
		company.Company{
			Classification: company.ClassificationProduct,
		},
	)

	if result.Recommendation == RecommendationSkip {
		t.Fatalf(
			"recommendation = SKIP, want shortlist: score=%.1f reasoning=%s",
			result.OverallScore,
			result.Reasoning,
		)
	}

	if result.ExperienceScore != 15 {
		t.Fatalf("experience score = %.1f, want 15", result.ExperienceScore)
	}
}

// An absent hard_ceiling_years must fall back to the candidate's own years, so
// forgetting the key tightens the filter rather than loosening it. The previous
// fallback sat above the candidate's experience, which meant an unset key
// silently admitted roles they could not apply to.
func TestExperienceHardCeilingFallsBackToCandidateYears(t *testing.T) {
	scorer := NewScorer(
		config.Candidate{
			Experience: config.Experience{TotalYears: 2},
		},
		testPreferences(),
	)

	if got := scorer.experienceHardCeiling(); got != 2 {
		t.Fatalf("ceiling = %.1f, want 2 (the candidate's total years)", got)
	}
}

// With neither the config key nor the candidate's years available, the veto must
// still be active rather than effectively unbounded.
func TestExperienceHardCeilingFallsBackToConstantWhenYearsUnknown(t *testing.T) {
	scorer := NewScorer(config.Candidate{}, testPreferences())

	if got := scorer.experienceHardCeiling(); got != defaultExperienceHardCeiling {
		t.Fatalf("ceiling = %.1f, want %.1f", got, defaultExperienceHardCeiling)
	}
}
