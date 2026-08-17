package scoring

import (
	"context"
	"fmt"
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

	// Saving the same job again should update the existing row,
	// not create a duplicate score.
	updated := result
	updated.OverallScore = 92
	updated.Recommendation = RecommendationShortlist
	updated.Reasoning = "updated score"

	if err := repo.Save(ctx, jobID, updated); err != nil {
		t.Fatal(err)
	}

	stored, err = repo.GetLatest(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatal("expected updated stored score")
	}

	if stored.OverallScore != 92 {
		t.Fatalf(
			"updated overall score = %.1f, want 92",
			stored.OverallScore,
		)
	}

	if stored.Recommendation != RecommendationShortlist {
		t.Fatalf(
			"updated recommendation = %q, want %q",
			stored.Recommendation,
			RecommendationShortlist,
		)
	}

	if stored.Reasoning != "updated score" {
		t.Fatalf(
			"updated reasoning = %q, want %q",
			stored.Reasoning,
			"updated score",
		)
	}

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM job_scores WHERE job_id = ?",
		jobID,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf(
			"job score row count = %d, want 1",
			count,
		)
	}
}

func TestRepositorySaveUpsertsExistingScore(t *testing.T) {
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
		SELECT id FROM job_sources WHERE name = ?
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
		"upsert-test",
		"Setu",
		"Backend Engineer",
		"Build backend systems",
		"Bengaluru",
		"https://example.com/job/upsert",
	)
	if err != nil {
		t.Fatal(err)
	}

	var jobID int64

	if err := db.QueryRow(`
		SELECT id FROM jobs WHERE external_id = ?
	`, "upsert-test").Scan(&jobID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	ctx := context.Background()

	first := Result{
		OverallScore:      60,
		SkillsScore:       10,
		RoleScore:         10,
		ExperienceScore:   10,
		DomainScore:       5,
		LocationScore:     10,
		CompanyScore:      5,
		CompensationScore: 5,
		Recommendation:    RecommendationSkip,
		Reasoning:         "first",
	}

	if err := repo.Save(ctx, jobID, first); err != nil {
		t.Fatal(err)
	}

	second := first
	second.OverallScore = 90
	second.Recommendation = RecommendationApply
	second.Reasoning = "second"

	if err := repo.Save(ctx, jobID, second); err != nil {
		t.Fatal(err)
	}

	var count int

	if err := db.QueryRow(`
		SELECT COUNT(*) FROM job_scores WHERE job_id = ?
	`, jobID).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("score count = %d, want 1", count)
	}

	stored, err := repo.GetLatest(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatal("expected stored score")
	}

	if stored.OverallScore != 90 {
		t.Fatalf("overall score = %.1f, want 90", stored.OverallScore)
	}

	if stored.Recommendation != RecommendationApply {
		t.Fatalf(
			"recommendation = %q, want %q",
			stored.Recommendation,
			RecommendationApply,
		)
	}

	if stored.Reasoning != "second" {
		t.Fatalf(
			"reasoning = %q, want %q",
			stored.Reasoning,
			"second",
		)
	}
}

func TestRepositoryListLatestOrdersByScore(t *testing.T) {
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

	var jobIDs []int64

	for i := 0; i < 3; i++ {
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
			fmt.Sprintf("rank-%d", i),
			fmt.Sprintf("Company %d", i),
			"Backend Engineer",
			"Backend systems",
			"Bengaluru",
			fmt.Sprintf("https://example.com/%d", i),
		)
		if err != nil {
			t.Fatal(err)
		}

		var jobID int64
		if err := db.QueryRow(`
			SELECT id
			FROM jobs
			WHERE external_id = ?
		`, fmt.Sprintf("rank-%d", i)).Scan(&jobID); err != nil {
			t.Fatal(err)
		}

		jobIDs = append(jobIDs, jobID)
	}

	repo := NewRepository(db)
	ctx := context.Background()

	scores := []float64{45, 80, 65}

	for i, score := range scores {
		result := Result{
			OverallScore:      score,
			SkillsScore:       score / 2,
			RoleScore:         10,
			ExperienceScore:   15,
			DomainScore:       5,
			LocationScore:     10,
			CompanyScore:      5,
			CompensationScore: 5,
			Recommendation:    RecommendationSkip,
			Reasoning:         fmt.Sprintf("score %.1f", score),
		}

		if err := repo.Save(ctx, jobIDs[i], result); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d scores, want 3", len(got))
	}

	want := []float64{80, 65, 45}

	for i, score := range want {
		if got[i].OverallScore != score {
			t.Fatalf(
				"score[%d] = %.1f, want %.1f",
				i,
				got[i].OverallScore,
				score,
			)
		}
	}
}
