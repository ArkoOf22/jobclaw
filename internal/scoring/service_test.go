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
