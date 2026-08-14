package company

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestServiceClassify(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	collector := NewEvidenceCollector(db)

	nameClassifier := NewRuleBasedClassifier()
	classifier := NewEvidenceClassifier(nameClassifier)

	service := NewService(
		repo,
		collector,
		classifier,
	)

	ctx := context.Background()

	company, err := repo.GetOrCreate(ctx, "Acme")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO job_sources (
			name,
			base_url
		)
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
		"Acme",
		company.ID,
		"Backend Engineer",
		"Build and operate our SaaS software platform.",
		"https://example.com/job/1",
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Classify(ctx, company.ID)
	if err != nil {
		t.Fatal(err)
	}

	if result.Classification != ClassificationProduct {
		t.Fatalf(
			"classification = %q, want %q",
			result.Classification,
			ClassificationProduct,
		)
	}

	if result.ClassificationReason == "" {
		t.Fatal("expected classification reason")
	}

	if result.ClassifiedAt == nil {
		t.Fatal("expected classified_at")
	}

	stored, err := repo.GetByID(ctx, company.ID)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Classification != ClassificationProduct {
		t.Fatalf(
			"stored classification = %q, want %q",
			stored.Classification,
			ClassificationProduct,
		)
	}

	if stored.ClassifiedAt == nil {
		t.Fatal("expected stored classified_at")
	}
}
