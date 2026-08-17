package main

import (
	"context"
	"fmt"
	"log"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

func main() {
	db, err := database.Open("data/jobclaw.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	repo := job.NewSQLiteRepository(db)

	jobs, err := repo.List(context.Background(), 20)
	if err != nil {
		log.Fatal(err)
	}

	for _, j := range jobs {
		fmt.Println("========================================")
		fmt.Printf("ID:       %d\n", j.ID)
		fmt.Printf("Company:  %s\n", j.Company)
		fmt.Printf("Title:    %s\n", j.Title)
		fmt.Printf("Location: %s\n", j.Location)
		fmt.Printf("URL:      %s\n", j.URL)
		fmt.Println()
		fmt.Println("DESCRIPTION:")
		fmt.Println(j.Description)
		fmt.Println()
	}
}
