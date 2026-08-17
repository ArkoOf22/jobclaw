package main

import (
	"context"
	"fmt"
	"log"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/scoring"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load(
		"config/candidate.yaml",
		"config/preferences.yaml",
	)
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Open("data/jobclaw.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	jobRepo := job.NewSQLiteRepository(db)
	companyRepo := company.NewRepository(db)
	scoreRepo := scoring.NewRepository(db)

	scorer := scoring.NewScorer(
		cfg.Candidate.Candidate,
		cfg.Preferences.JobPreferences,
	)

	service := scoring.NewService(
		jobRepo,
		companyRepo,
		scoreRepo,
		scorer,
	)

	jobs, err := jobRepo.List(ctx, 1000)
	if err != nil {
		log.Fatal(err)
	}

	for _, j := range jobs {
		if _, err := service.ScoreJob(ctx, j.ID); err != nil {
			log.Printf(
				"score job %d (%s): %v",
				j.ID,
				j.Title,
				err,
			)
		}
	}

	scores, err := scoreRepo.List(ctx, 1000)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("          JobClaw Job Ranking")
	fmt.Println("========================================")
	fmt.Println()

	for i, score := range scores {
		j, err := jobRepo.GetByID(ctx, score.JobID)
		if err != nil {
			log.Printf(
				"load job %d: %v",
				score.JobID,
				err,
			)
			continue
		}

		if j == nil {
			continue
		}

		fmt.Printf(
			"%d. %s — %s\n",
			i+1,
			j.Company,
			j.Title,
		)

		fmt.Printf(
			"   Score: %.1f | Recommendation: %s\n",
			score.OverallScore,
			score.Recommendation,
		)

		fmt.Printf(
			"   %s\n",
			score.Reasoning,
		)

		fmt.Printf(
			"   %s\n",
			j.URL,
		)

		fmt.Println()
	}

	var apply, shortlist, skip int

	for _, score := range scores {
		switch score.Recommendation {
		case scoring.RecommendationApply:
			apply++
		case scoring.RecommendationShortlist:
			shortlist++
		case scoring.RecommendationSkip:
			skip++
		}
	}

	fmt.Println("========================================")
	fmt.Println("Summary")
	fmt.Println("========================================")
	fmt.Printf("Total:      %d\n", len(scores))
	fmt.Printf("APPLY:      %d\n", apply)
	fmt.Printf("SHORTLIST:  %d\n", shortlist)
	fmt.Printf("SKIP:       %d\n", skip)
}
