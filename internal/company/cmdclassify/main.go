package main

import (
	"context"
	"fmt"
	"log"

	"jobclaw/internal/company"
	"jobclaw/internal/database"
)

type targetCompany struct {
	ID   int64
	Name string
}

func main() {
	db, err := database.Open("data/jobclaw.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	repo := company.NewRepository(db)
	collector := company.NewEvidenceCollector(db)
	nameClassifier := company.NewRuleBasedClassifier()
	classifier := company.NewEvidenceClassifier(nameClassifier)
	service := company.NewService(repo, collector, classifier)

	ctx := context.Background()

	rows, err := db.Query(`
		SELECT id, name
		FROM companies
		WHERE classification = ?
		ORDER BY id
	`, company.ClassificationUnknown)
	if err != nil {
		log.Fatal(err)
	}

	var companies []targetCompany

	for rows.Next() {
		var c targetCompany

		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			rows.Close()
			log.Fatal(err)
		}

		companies = append(companies, c)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		log.Fatal(err)
	}

	if err := rows.Close(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"Companies requiring classification: %d\n\n",
		len(companies),
	)

	processed := 0

	for _, target := range companies {
		result, err := service.Classify(ctx, target.ID)
		if err != nil {
			log.Fatalf(
				"classify company %d (%q): %v",
				target.ID,
				target.Name,
				err,
			)
		}

		fmt.Printf(
			"%s -> %s\nReason: %s\n\n",
			result.Name,
			result.Classification,
			result.ClassificationReason,
		)

		processed++
	}

	fmt.Printf(
		"Classification complete. Companies processed: %d\n",
		processed,
	)
}
