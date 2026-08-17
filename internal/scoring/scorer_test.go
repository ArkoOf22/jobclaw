package scoring

import (
	"testing"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/job"
)

func testPreferences() config.JobPreferences {
	return config.JobPreferences{
		CompanyType: config.CompanyType{
			RequireProductCompany: true,
		},
		Roles: config.Roles{
			Preferred:  []string{"Backend Engineer", "Software Engineer"},
			Acceptable: []string{"Platform Engineer"},
			Excluded:   []string{"Frontend Engineer"},
		},
		Locations: config.Locations{
			Preferred:  []string{"Bengaluru", "Bangalore"},
			Acceptable: []string{"Hyderabad", "Pune"},
		},
		Compensation: config.Compensation{
			Currency:     "INR",
			TargetMinLPA: 21,
			TargetMaxLPA: 23,
		},
		Domains: config.Domains{
			StronglyPreferred: []string{
				"Fintech",
				"Payments",
				"SaaS",
				"Distributed Systems",
			},
			Preferred: []string{"Cybersecurity"},
		},
		TechnologyPreferences: config.TechnologyPreferences{
			StronglyPreferred: []string{
				"Go",
				"Kafka",
				"Redis",
				"Backend",
				"PostgreSQL",
				"Concurrency",
			},
			Preferred: []string{
				"Java",
				"AWS",
				"Docker",
			},
		},
		JobQuality: config.JobQuality{
			MinimumScoreForShortlist: 70,
			MinimumScoreForApply:     80,
		},
	}
}

func TestScorerStrongBackendRole(t *testing.T) {
	scorer := NewScorer(
		config.Candidate{
			Experience: config.Experience{
				TotalYears: 2,
			},
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
			},
			DomainExperience: []string{
				"SaaS",
				"Distributed Systems",
			},
		},
		testPreferences(),
	)

	result := scorer.Score(
		job.Job{
			Title:       "Backend Engineer",
			Description: "Build backend services in Go with Kafka, Redis and concurrency for a SaaS distributed systems platform. 2 years experience.",
			Location:    "Bengaluru, Karnataka",
		},
		company.Company{
			Classification: company.ClassificationProduct,
		},
	)

	if result.OverallScore < 70 {
		t.Fatalf("overall score = %.1f, expected >= 70", result.OverallScore)
	}

	if result.Recommendation == RecommendationSkip {
		t.Fatalf("expected shortlist/apply, got %s", result.Recommendation)
	}

	if result.SkillsScore <= 0 {
		t.Fatal("expected skills score")
	}

	if result.CompanyScore != 5 {
		t.Fatalf("company score = %.1f, want 5", result.CompanyScore)
	}

	if result.LocationScore != 10 {
		t.Fatalf("location score = %.1f, want 10", result.LocationScore)
	}
}

func TestScorerServicesCompanyIsSkipped(t *testing.T) {
	scorer := NewScorer(
		config.Candidate{},
		testPreferences(),
	)

	result := scorer.Score(
		job.Job{
			Title:       "Software Engineer",
			Description: "Build backend services in Go.",
			Location:    "Bengaluru",
		},
		company.Company{
			Classification: company.ClassificationServices,
		},
	)

	if result.Recommendation != RecommendationSkip {
		t.Fatalf(
			"recommendation = %s, want %s",
			result.Recommendation,
			RecommendationSkip,
		)
	}
}

func TestScorerExcludedRole(t *testing.T) {
	scorer := NewScorer(
		config.Candidate{},
		testPreferences(),
	)

	result := scorer.Score(
		job.Job{
			Title:    "Frontend Engineer",
			Location: "Bengaluru",
		},
		company.Company{
			Classification: company.ClassificationProduct,
		},
	)

	if result.Recommendation != RecommendationSkip {
		t.Fatalf(
			"recommendation = %s, want %s",
			result.Recommendation,
			RecommendationSkip,
		)
	}
}
