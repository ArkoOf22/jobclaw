package company

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestEvidenceCollector(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	companyRepo := NewRepository(db)

	companyRecord, err := companyRepo.GetOrCreate(
		context.Background(),
		"Adobe",
	)
	if err != nil {
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
			company_id,
			title,
			description,
			url
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		sourceID,
		"test-1",
		"Adobe",
		companyRecord.ID,
		"Software Engineer",
		"Build and scale our cloud platform and software product.",
		"https://example.com/job/1",
	)
	if err != nil {
		t.Fatal(err)
	}

	collector := NewEvidenceCollector(db)

	evidence, err := collector.Collect(
		context.Background(),
		companyRecord.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if evidence.JobCount != 1 {
		t.Fatalf(
			"job count = %d, want 1",
			evidence.JobCount,
		)
	}

	if len(evidence.Titles) != 1 {
		t.Fatalf(
			"title count = %d, want 1",
			len(evidence.Titles),
		)
	}

	if len(evidence.ProductSignals) == 0 {
		t.Fatal("expected product signals")
	}
}
