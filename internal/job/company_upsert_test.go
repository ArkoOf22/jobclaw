package job

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestSQLiteRepositoryUpsertCreatesCompanyLink(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewSQLiteRepository(db)

	err = repo.Upsert(context.Background(), Job{
		Source:         "linkedin",
		ExternalID:     "linkedin:test-123",
		Company:        "Adobe",
		Title:          "Software Development Engineer",
		Description:    "Backend engineering role",
		Location:       "Bengaluru",
		RemoteType:     "ONSITE",
		EmploymentType: "fulltime",
		URL:            "https://example.com/job/test-123",
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		companyID      int64
		companyName    string
		classification string
	)

	err = db.QueryRow(`
		SELECT
			c.id,
			c.name,
			c.classification
		FROM jobs j
		JOIN companies c ON c.id = j.company_id
		WHERE j.external_id = ?
	`, "linkedin:test-123").Scan(
		&companyID,
		&companyName,
		&classification,
	)
	if err != nil {
		t.Fatal(err)
	}

	if companyID == 0 {
		t.Fatal("expected company ID")
	}

	if companyName != "Adobe" {
		t.Fatalf("company name = %q, want %q", companyName, "Adobe")
	}

	if classification != "UNKNOWN" {
		t.Fatalf(
			"classification = %q, want %q",
			classification,
			"UNKNOWN",
		)
	}
}
