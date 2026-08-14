package database

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func (db *DB) MigrateEmbedded() error {
	if err := db.ensureMigrationTable(); err != nil {
		return err
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var migrations []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".sql") {
			migrations = append(migrations, name)
		}
	}

	sort.Strings(migrations)

	for _, name := range migrations {
		applied, err := db.isMigrationApplied(name)
		if err != nil {
			return err
		}

		if applied {
			continue
		}

		migration, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read embedded migration %q: %w", name, err)
		}

		if err := db.executeMigration(name, string(migration)); err != nil {
			return err
		}

		if _, err := db.Exec(
			`INSERT INTO schema_migrations (name) VALUES (?)`,
			name,
		); err != nil {
			return fmt.Errorf("record migration %q: %w", name, err)
		}
	}

	return nil
}

func (db *DB) ensureMigrationTable() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create migration tracking table: %w", err)
	}

	return nil
}

func (db *DB) isMigrationApplied(name string) (bool, error) {
	var exists int

	err := db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM schema_migrations WHERE name = ?
		)`,
		name,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("check migration %q: %w", name, err)
	}

	return exists == 1, nil
}

func (db *DB) executeMigration(name, migration string) error {
	statements := strings.Split(migration, ";")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}

	for _, statement := range statements {
		statement = strings.TrimSpace(statement)

		if statement == "" {
			continue
		}

		if _, err := tx.Exec(statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration %q: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}

	return nil
}
