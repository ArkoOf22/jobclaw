package main

import (
	"fmt"
	"log"
	"os"

	"jobclaw/internal/database"
)

const defaultDatabasePath = "data/jobclaw.db"

func main() {
	databasePath := defaultDatabasePath

	if value := os.Getenv("JOBCLAW_DB_PATH"); value != "" {
		databasePath = value
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
	fmt.Println("Database:", databasePath)
	fmt.Println("Status: READY")
}
