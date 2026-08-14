package company

import (
	"context"
	"testing"

	"jobclaw/internal/database"
)

func TestRepositoryGetOrCreate(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	ctx := context.Background()

	first, err := repo.GetOrCreate(ctx, "Adobe")
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == 0 {
		t.Fatal("expected company ID")
	}

	if first.NormalizedName != "adobe" {
		t.Fatalf(
			"normalized name = %q, want %q",
			first.NormalizedName,
			"adobe",
		)
	}

	second, err := repo.GetOrCreate(ctx, "  ADOBE  ")
	if err != nil {
		t.Fatal(err)
	}

	if second.ID != first.ID {
		t.Fatalf(
			"expected same company ID, got %d and %d",
			first.ID,
			second.ID,
		)
	}

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM companies`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("company count = %d, want 1", count)
	}
}
