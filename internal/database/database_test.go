package database

import (
	"os"
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

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table'
		AND name NOT LIKE 'sqlite_%'
	`).Scan(&count)

	if err != nil {
		t.Fatalf("count tables: %v", err)
	}

	if count != 7 {
		t.Fatalf("expected 7 tables, got %d", count)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file does not exist: %v", err)
	}
}
