package main

import (
	"context"
	"fmt"
	"log"
	"sort"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/scoring"
)

type scoredJob struct {
	Job    job.Job
	Result scoring.Result
}

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

	var scored []scoredJob

	for _, j := range jobs {
		result, err := service.ScoreJob(ctx, j.ID)
		if err != nil {
			log.Printf(
				"skip job %d (%s): %v",
				j.ID,
				j.Title,
				err,
			)
			continue
		}

		scored = append(scored, scoredJob{
			Job:    j,
			Result: *result,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Result.OverallScore >
			scored[j].Result.OverallScore
	})

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("          JobClaw Job Ranking")
	fmt.Println("========================================")
	fmt.Println()

	for i, item := range scored {
		fmt.Printf(
			"%d. %s — %s\n",
			i+1,
			item.Job.Company,
			item.Job.Title,
		)

		fmt.Printf(
			"   Score: %.1f | Recommendation: %s\n",
			item.Result.OverallScore,
			item.Result.Recommendation,
		)

		fmt.Printf(
			"   %s\n",
			item.Result.Reasoning,
		)

		fmt.Printf(
			"   %s\n",
			item.Job.URL,
		)

		fmt.Println()
	}

	var apply, shortlist, skip int

	for _, item := range scored {
		switch item.Result.Recommendation {
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
	fmt.Printf("Total:      %d\n", len(scored))
	fmt.Printf("APPLY:      %d\n", apply)
	fmt.Printf("SHORTLIST:  %d\n", shortlist)
	fmt.Printf("SKIP:       %d\n", skip)
}
