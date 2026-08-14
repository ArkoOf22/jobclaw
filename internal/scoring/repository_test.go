package scoring

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestRepositorySaveAndGetLatest(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
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

	_, err = db.Exec(`
		INSERT INTO jobs (
			source_id,
			external_id,
			company,
			title,
			description,
			location,
			url
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		sourceID,
		"test-1",
		"Adobe",
		"Backend Engineer",
		"Build backend systems",
		"Bengaluru",
		"https://example.com/job/1",
	)
	if err != nil {
		t.Fatal(err)
	}

	var jobID int64

	if err := db.QueryRow(`
		SELECT id
		FROM jobs
		WHERE external_id = ?
	`, "test-1").Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)

	result := Result{
		OverallScore:      87,
		SkillsScore:       27,
		RoleScore:         15,
		ExperienceScore:   15,
		DomainScore:       7,
		LocationScore:     10,
		CompanyScore:      10,
		CompensationScore: 3,
		Recommendation:    RecommendationApply,
		Reasoning:         "test score",
	}

	ctx := context.Background()

	if err := repo.Save(ctx, jobID, result); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.GetLatest(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatal("expected stored score")
	}

	if stored.OverallScore != 87 {
		t.Fatalf(
			"overall score = %.1f, want 87",
			stored.OverallScore,
		)
	}

	if stored.Recommendation != RecommendationApply {
		t.Fatalf(
			"recommendation = %q, want %q",
			stored.Recommendation,
			RecommendationApply,
		)
	}

	if stored.Reasoning != "test score" {
		t.Fatalf(
			"reasoning = %q, want %q",
			stored.Reasoning,
			"test score",
		)
	}
}
