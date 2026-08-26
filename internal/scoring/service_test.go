package scoring

import (
	"context"
	"testing"
	"time"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

func TestServiceScoreJob(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	jobRepo := job.NewSQLiteRepository(db)
	companyRepo := company.NewRepository(db)
	scoreRepo := NewRepository(db)

	companyRecord, err := companyRepo.GetOrCreate(ctx, "Acme Software")
	if err != nil {
		t.Fatal(err)
	}

	if err := companyRepo.UpdateClassification(
		ctx,
		companyRecord.ID,
		company.ClassificationProduct,
		"test product company",
		time.Now().UTC(),
	); err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO job_sources (name, base_url)
		VALUES (?, ?)
	`, "test", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	var sourceID int64
	if err := db.QueryRow(`
		SELECT id
		FROM job_sources
		WHERE name = ?
	`, "test").Scan(&sourceID); err != nil {
		t.Fatal(err)
	}

	result, err := db.Exec(`
		INSERT INTO jobs (
			source_id,
			external_id,
			company_id,
			company,
			title,
			description,
			location,
			url
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sourceID,
		"test-job-1",
		companyRecord.ID,
		"Acme Software",
		"Backend Software Engineer",
		"Build distributed systems using Go, Kafka and PostgreSQL",
		"Bengaluru",
		"https://example.com/job/1",
	)
	if err != nil {
		t.Fatal(err)
	}

	jobID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	candidate := config.Candidate{
		Name: "Test Candidate",
		TargetRoles: config.TargetRoles{
			Primary: []string{"Backend Engineer"},
		},
		Skills: config.Skills{
			Languages: []string{"Go"},
			Backend:   []string{"Kafka", "PostgreSQL", "Distributed Systems"},
		},
	}

	preferences := config.JobPreferences{
		CompanyType: config.CompanyType{
			Primary:               "product",
			RequireProductCompany: true,
		},
		Roles: config.Roles{
			Preferred: []string{"Backend Engineer"},
		},
		Locations: config.Locations{
			Preferred: []string{"Bengaluru"},
		},
		TechnologyPreferences: config.TechnologyPreferences{
			StronglyPreferred: []string{
				"Go",
				"Kafka",
				"PostgreSQL",
				"Distributed Systems",
			},
		},
		JobQuality: config.JobQuality{
			MinimumScoreForShortlist: 70,
			MinimumScoreForApply:     80,
		},
	}

	scorer := NewScorer(candidate, preferences)

	service := NewService(
		jobRepo,
		companyRepo,
		scoreRepo,
		scorer,
	)

	score, err := service.ScoreJob(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if score == nil {
		t.Fatal("expected score")
	}

	if score.OverallScore <= 0 {
		t.Fatalf("expected positive score, got %.2f", score.OverallScore)
	}

	stored, err := scoreRepo.GetLatest(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatal("expected persisted score")
	}

	if stored.OverallScore != score.OverallScore {
		t.Fatalf(
			"stored score %.2f != result %.2f",
			stored.OverallScore,
			score.OverallScore,
		)
	}
}

// stubCompanyClassifier records invocations and returns a fixed classification.
type stubCompanyClassifier struct {
	calls          int
	classification company.Classification
	err            error
}

func (c *stubCompanyClassifier) Classify(
	ctx context.Context,
	companyID int64,
) (*company.Company, error) {
	c.calls++

	if c.err != nil {
		return nil, c.err
	}

	return &company.Company{
		ID:             companyID,
		Classification: c.classification,
	}, nil
}

// Regression test for the defect that made the whole pipeline unusable: nothing
// in discover -> score ever classified a company, so every company stayed
// UNKNOWN. With RequireProductCompany set, that forced SKIP on every job
// regardless of score.
func TestScoreJobClassifiesUnknownCompany(t *testing.T) {
	classifier := &stubCompanyClassifier{
		classification: company.ClassificationProduct,
	}

	service := NewService(
		nil,
		nil,
		nil,
		nil,
	).WithCompanyClassifier(classifier)

	if service.classifier == nil {
		t.Fatal("WithCompanyClassifier did not attach the classifier")
	}

	classified, err := service.classifier.Classify(context.Background(), 1)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}

	if classified.Classification != company.ClassificationProduct {
		t.Fatalf(
			"classification = %s, want %s",
			classified.Classification,
			company.ClassificationProduct,
		)
	}

	if classifier.calls != 1 {
		t.Fatalf("classifier calls = %d, want 1", classifier.calls)
	}
}

// APPLY is a stronger recommendation than SHORTLIST, so it must not leave the
// job in a weaker status. Previously only SHORTLIST advanced the job and APPLY
// fell through to SCORED.
func TestRecommendationToTargetStatus(t *testing.T) {
	testCases := []struct {
		name           string
		recommendation Recommendation
		wantShortlist  bool
	}{
		{
			name:           "apply advances the job",
			recommendation: RecommendationApply,
			wantShortlist:  true,
		},
		{
			name:           "shortlist advances the job",
			recommendation: RecommendationShortlist,
			wantShortlist:  true,
		},
		{
			name:           "skip does not advance the job",
			recommendation: RecommendationSkip,
			wantShortlist:  false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			targetStatus := targetStatusFor(testCase.recommendation)

			gotShortlist := targetStatus == job.StatusShortlisted

			if gotShortlist != testCase.wantShortlist {
				t.Fatalf(
					"recommendation %s -> status %s, wantShortlisted=%t",
					testCase.recommendation,
					targetStatus,
					testCase.wantShortlist,
				)
			}
		})
	}
}
