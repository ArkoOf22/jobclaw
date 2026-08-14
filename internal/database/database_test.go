package database

import (
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "jobclaw.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	expectedTables := []string{
		"job_sources",
		"candidates",
		"jobs",
		"job_scores",
		"resumes",
		"applications",
		"application_events",
	}

	for _, table := range expectedTables {
		var count int

		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM sqlite_master
			WHERE type = 'table'
			AND name = ?
		`, table).Scan(&count)

		if err != nil {
			t.Fatalf("check table %q: %v", table, err)
		}

		if count != 1 {
			t.Errorf("expected table %q to exist", table)
		}
	}
}
