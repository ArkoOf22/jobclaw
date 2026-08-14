package database

import (
	"fmt"
	"os"
	"strings"
)

func (db *DB) Migrate(migrationPath string) error {
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	statements := strings.Split(string(migration), ";")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}

	for _, statement := range statements {
		statement = strings.TrimSpace(statement)

		if statement == "" {
			continue
		}

		if _, err := tx.Exec(statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration statement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	return nil
}
