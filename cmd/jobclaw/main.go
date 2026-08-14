package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"jobclaw/internal/config"
	"jobclaw/internal/database"
)

const (
	defaultDatabasePath    = "data/jobclaw.db"
	defaultCandidateConfig = "config/candidate.yaml"
	defaultPreferences     = "config/preferences.yaml"
)

func main() {
	databasePath := getEnvOrDefault("JOBCLAW_DB_PATH", defaultDatabasePath)
	candidateConfigPath := getEnvOrDefault(
		"JOBCLAW_CANDIDATE_CONFIG",
		defaultCandidateConfig,
	)
	preferencesPath := getEnvOrDefault(
		"JOBCLAW_PREFERENCES_CONFIG",
		defaultPreferences,
	)

	if err := validateConfigFiles(
		candidateConfigPath,
		preferencesPath,
	); err != nil {
		log.Fatalf("configuration initialization failed: %v", err)
	}

	cfg, err := config.Load(candidateConfigPath, preferencesPath)
	if err != nil {
		log.Fatalf("configuration initialization failed: %v", err)
	}

	db, err := database.Open(databasePath)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer db.Close()

	if err := db.MigrateEmbedded(); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("database health check failed: %v", err)
	}

	fmt.Println("JobClaw v0.1")
	fmt.Println("Candidate:", cfg.Candidate.Candidate.Name)
	fmt.Println("Target company type:", cfg.Preferences.JobPreferences.CompanyType.Primary)
	fmt.Println("Database:", databasePath)
	fmt.Println("Status: READY")
}

func validateConfigFiles(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(filepath.Clean(path)); err != nil {
			return fmt.Errorf("config file %q: %w", path, err)
		}
	}

	return nil
}

func getEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
