package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"jobclaw/internal/config"
	"jobclaw/internal/database"
	"jobclaw/internal/discovery"
	"jobclaw/internal/discovery/jobspy"
	"jobclaw/internal/job"
)

const (
	defaultDatabasePath    = "data/jobclaw.db"
	defaultCandidateConfig = "config/candidate.yaml"
	defaultPreferences     = "config/preferences.yaml"
	defaultJobSpyURL       = "http://127.0.0.1:8000/mcp"
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

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "discover":
			runDiscover(databasePath, cfg, db)
			return
		case "jobs":
			if len(os.Args) > 2 && os.Args[2] == "list" {
				runJobsList(db)
				return
			}
			log.Fatal("usage: jobclaw jobs list")
		}
	}

	fmt.Println("JobClaw v0.1")
	fmt.Println("Candidate:", cfg.Candidate.Candidate.Name)
	fmt.Println("Target company type:", cfg.Preferences.JobPreferences.CompanyType.Primary)
	fmt.Println("Database:", databasePath)
	fmt.Println("Status: READY")
}

func runDiscover(
	databasePath string,
	cfg *config.Config,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	repository := job.NewSQLiteRepository(db)

	jobSpyURL := getEnvOrDefault(
		"JOBCLAW_JOBSPY_URL",
		defaultJobSpyURL,
	)

	jobSpyClient := jobspy.NewClient(jobSpyURL)

	service := discovery.NewService(
		repository,
		jobSpyClient,
	)

	keywords := cfg.Candidate.Candidate.TargetRoles.Primary
	if len(keywords) == 0 {
		keywords = []string{"Backend Engineer", "Golang"}
	}

	locations := cfg.Preferences.JobPreferences.Locations.Preferred
	if len(locations) == 0 {
		locations = []string{"Bengaluru"}
	}

	results := service.Discover(ctx, discovery.Request{
		Keywords:   keywords,
		Locations:  locations,
		RemoteOnly: false,
		HoursOld:   168,
		Limit:      10,
	})

	fmt.Println("JobClaw Discovery")
	fmt.Println("────────────────────────────")
	fmt.Println("Candidate:", cfg.Candidate.Candidate.Name)
	fmt.Println("Database:", databasePath)
	fmt.Println()

	for _, result := range results {
		fmt.Printf(
			"Source: %s\nFetched: %d\nStored: %d\nDuration: %s\n",
			result.Source,
			result.Fetched,
			result.Stored,
			result.Duration.Round(time.Millisecond),
		)

		if result.Failed {
			fmt.Println("Status: FAILED")
			fmt.Println("Error:", result.Error)
		} else {
			fmt.Println("Status: OK")
		}

		fmt.Println()
	}
}

func runJobsList(db *database.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := job.NewSQLiteRepository(db)

	jobs, err := repository.List(ctx, 20)
	if err != nil {
		log.Fatalf("list jobs: %v", err)
	}

	fmt.Println("JobClaw Jobs")
	fmt.Println("────────────────────────────")
	fmt.Printf("Stored jobs: %d\n\n", len(jobs))

	for i, j := range jobs {
		fmt.Printf(
			"%d. [%s] %s — %s\n   %s\n   %s\n\n",
			i+1,
			j.Source,
			j.Company,
			j.Title,
			j.Location,
			j.URL,
		)
	}
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
