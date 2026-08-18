package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jobclaw/internal/application"
	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/database"
	"jobclaw/internal/discovery"
	"jobclaw/internal/discovery/jobspy"
	"jobclaw/internal/job"
	"jobclaw/internal/llm/openrouter"
	"jobclaw/internal/scoring"
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

		case "score":
			runScore(databasePath, cfg, db)
			return

		case "shortlist":
			runShortlist(db)
			return

		case "approve":
			if len(os.Args) != 3 {
				log.Fatal("usage: jobclaw approve <id>")
			}

			jobID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || jobID <= 0 {
				log.Fatal("job ID must be a positive integer")
			}

			runStatusChange(jobID, job.StatusApproved, db)
			return

		case "reject":
			if len(os.Args) != 3 {
				log.Fatal("usage: jobclaw reject <id>")
			}

			jobID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || jobID <= 0 {
				log.Fatal("job ID must be a positive integer")
			}

			runStatusChange(jobID, job.StatusRejected, db)
			return

		case "application":
			if len(os.Args) != 3 {
				log.Fatal("usage: jobclaw application <id>")
			}

			jobID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || jobID <= 0 {
				log.Fatal("job ID must be a positive integer")
			}

			runApplication(jobID, cfg, db)
			return

		case "resume":
			if len(os.Args) != 3 {
				log.Fatal("usage: jobclaw resume <id>")
			}

			jobID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || jobID <= 0 {
				log.Fatal("job ID must be a positive integer")
			}

			runResume(jobID, cfg, db)
			return

		case "job":
			if len(os.Args) != 3 {
				log.Fatal("usage: jobclaw job <id>")
			}

			jobID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || jobID <= 0 {
				log.Fatal("job ID must be a positive integer")
			}

			runJob(jobID, db)
			return
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

func runResume(
	jobID int64,
	cfg *config.Config,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)

	resumeConfig := cfg.Resume.Resume

	apiKey := os.Getenv(resumeConfig.LLM.APIKeyEnv)
	if apiKey == "" {
		log.Fatalf(
			"resume LLM API key environment variable %q is not set",
			resumeConfig.LLM.APIKeyEnv,
		)
	}

	resumeSource := application.NewResumeSource(
		application.ResumeSourceConfig{
			MasterPath:            resumeConfig.MasterPath,
			OutputFormat:          resumeConfig.OutputFormat,
			PreserveFacts:         resumeConfig.Generation.PreserveFacts,
			AllowRewording:        resumeConfig.Generation.AllowRewording,
			AllowReordering:       resumeConfig.Generation.AllowReordering,
			AllowSkillSelection:   resumeConfig.Generation.AllowSkillSelection,
			AllowMetricChanges:    resumeConfig.Generation.AllowMetricChanges,
			AllowExperienceInvent: resumeConfig.Generation.AllowExperienceInvention,
		},
	)

	promptBuilder := application.NewResumePromptBuilder(resumeSource)

	llmClient, err := openrouter.NewClient(
		openrouter.Config{
			APIKey:  apiKey,
			Model:   resumeConfig.LLM.Model,
			BaseURL: resumeConfig.LLM.BaseURL,
		},
	)
	if err != nil {
		log.Fatalf("initialize resume LLM: %v", err)
	}

	generator := application.NewLLMResumeGenerator(
		jobRepo,
		promptBuilder,
		llmClient,
	)

	outputPath := filepath.Join(
		"data",
		"applications",
		strconv.FormatInt(jobID, 10),
		"resume",
		"tailored_resume.txt",
	)

	if err := generator.GenerateTailoredResume(
		ctx,
		jobID,
		outputPath,
	); err != nil {
		log.Fatalf("generate tailored resume: %v", err)
	}

	fmt.Println("JobClaw Resume")
	fmt.Println("────────────────────────────")
	fmt.Printf("Job ID:     %d\n", jobID)
	fmt.Printf("Model:      %s\n", resumeConfig.LLM.Model)
	fmt.Printf("Output:     %s\n", outputPath)
}

func runApplication(jobID int64, cfg *config.Config, db *database.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)
	applicationRepo := application.NewSQLiteRepository(db)
	eventRepo := application.NewSQLiteEventRepository(db)

	resumeConfig := cfg.Resume.Resume

	apiKey := os.Getenv(resumeConfig.LLM.APIKeyEnv)
	if apiKey == "" {
		log.Fatalf(
			"resume LLM API key environment variable %q is not set",
			resumeConfig.LLM.APIKeyEnv,
		)
	}

	resumeSource := application.NewResumeSource(
		application.ResumeSourceConfig{
			MasterPath:            resumeConfig.MasterPath,
			OutputFormat:          resumeConfig.OutputFormat,
			PreserveFacts:         resumeConfig.Generation.PreserveFacts,
			AllowRewording:        resumeConfig.Generation.AllowRewording,
			AllowReordering:       resumeConfig.Generation.AllowReordering,
			AllowSkillSelection:   resumeConfig.Generation.AllowSkillSelection,
			AllowMetricChanges:    resumeConfig.Generation.AllowMetricChanges,
			AllowExperienceInvent: resumeConfig.Generation.AllowExperienceInvention,
		},
	)

	promptBuilder := application.NewResumePromptBuilder(resumeSource)

	llmClient, err := openrouter.NewClient(
		openrouter.Config{
			APIKey:  apiKey,
			Model:   resumeConfig.LLM.Model,
			BaseURL: resumeConfig.LLM.BaseURL,
		},
	)
	if err != nil {
		log.Fatalf("initialize resume LLM: %v", err)
	}

	resumeGenerator := application.NewLLMResumeGenerator(
		jobRepo,
		promptBuilder,
		llmClient,
	)

	service := application.NewService(
		jobRepo,
		applicationRepo,
		eventRepo,
		resumeGenerator,
	)

	app, err := service.CreateForApprovedJob(ctx, jobID)
	if err != nil {
		log.Fatalf("create application: %v", err)
	}

	fmt.Println("JobClaw Application")
	fmt.Println("────────────────────────────")
	fmt.Printf("Application ID: %d\n", app.ID)
	fmt.Printf("Job ID:         %d\n", app.JobID)
	fmt.Printf("Status:         %s\n", app.Status)
	fmt.Println()
	fmt.Println("Application workspace:")
	fmt.Printf("data/applications/%d/\n", app.JobID)
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

func runScore(
	databasePath string,
	cfg *config.Config,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
		log.Fatalf("list jobs: %v", err)
	}

	fmt.Println("JobClaw Scoring")
	fmt.Println("────────────────────────────")
	fmt.Printf("Jobs: %d\n\n", len(jobs))

	scoredCount := 0

	for _, j := range jobs {
		if _, err := service.ScoreJob(ctx, j.ID); err != nil {
			log.Printf(
				"skip job %d (%s): %v",
				j.ID,
				j.Title,
				err,
			)
			continue
		}

		scoredCount++
	}

	scores, err := scoreRepo.List(ctx, scoredCount)
	if err != nil {
		log.Fatalf("list scores: %v", err)
	}

	jobByID := make(map[int64]job.Job, len(jobs))
	for _, j := range jobs {
		jobByID[j.ID] = j
	}

	for i, score := range scores {
		j, ok := jobByID[score.JobID]
		if !ok {
			continue
		}

		fmt.Printf(
			"#%d  %s — %s\n",
			i+1,
			j.Company,
			j.Title,
		)
		fmt.Printf(
			"    Score: %.1f | %s\n",
			score.OverallScore,
			score.Recommendation,
		)
		fmt.Printf(
			"    %s\n",
			score.Reasoning,
		)
		fmt.Printf(
			"    %s\n\n",
			j.URL,
		)
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

	fmt.Println("────────────────────────────")
	fmt.Println("Summary")
	fmt.Println("────────────────────────────")
	fmt.Printf("Scored:     %d\n", len(scores))
	fmt.Printf("APPLY:      %d\n", apply)
	fmt.Printf("SHORTLIST:  %d\n", shortlist)
	fmt.Printf("SKIP:       %d\n", skip)
	fmt.Println()
	fmt.Println("Database:", databasePath)
}

func runShortlist(db *database.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)
	scoreRepo := scoring.NewRepository(db)

	scores, err := scoreRepo.List(ctx, 1000)
	if err != nil {
		log.Fatalf("list scores: %v", err)
	}

	fmt.Println("JobClaw Shortlist")
	fmt.Println("────────────────────────────")
	fmt.Println()

	count := 0

	for _, score := range scores {
		if score.Recommendation != scoring.RecommendationShortlist &&
			score.Recommendation != scoring.RecommendationApply {
			continue
		}

		j, err := jobRepo.GetByID(ctx, score.JobID)
		if err != nil {
			log.Printf(
				"skip job %d: %v",
				score.JobID,
				err,
			)
			continue
		}

		if j == nil {
			continue
		}

		count++

		fmt.Printf(
			"#%d  %s — %s\n",
			count,
			j.Company,
			j.Title,
		)
		fmt.Printf(
			"    Score: %.1f | %s\n",
			score.OverallScore,
			score.Recommendation,
		)
		fmt.Printf(
			"    Location: %s\n",
			j.Location,
		)
		fmt.Printf(
			"    %s\n\n",
			j.URL,
		)
	}

	fmt.Println("────────────────────────────")
	fmt.Printf("Shortlisted: %d\n", count)
}

func runJob(jobID int64, db *database.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)
	scoreRepo := scoring.NewRepository(db)

	j, err := jobRepo.GetByID(ctx, jobID)
	if err != nil {
		log.Fatalf("load job: %v", err)
	}

	if j == nil {
		log.Fatalf("job %d not found", jobID)
	}

	score, err := scoreRepo.GetLatest(ctx, jobID)
	if err != nil {
		log.Fatalf("load job score: %v", err)
	}

	fmt.Println("JobClaw Job")
	fmt.Println("────────────────────────────")
	fmt.Printf("ID:       %d\n", j.ID)
	fmt.Printf("Company:  %s\n", j.Company)
	fmt.Printf("Title:    %s\n", j.Title)
	fmt.Printf("Location: %s\n", j.Location)
	fmt.Printf("Source:   %s\n", j.Source)
	fmt.Printf("URL:      %s\n", j.URL)

	if j.RemoteType != "" {
		fmt.Printf("Remote:   %s\n", j.RemoteType)
	}

	if j.EmploymentType != "" {
		fmt.Printf("Type:     %s\n", j.EmploymentType)
	}

	fmt.Println()

	if j.SalaryMin != nil || j.SalaryMax != nil {
		fmt.Println("Compensation")
		fmt.Println("────────────────────────────")

		switch {
		case j.SalaryMin != nil && j.SalaryMax != nil:
			fmt.Printf(
				"%s%d - %d\n",
				j.Currency,
				*j.SalaryMin,
				*j.SalaryMax,
			)
		case j.SalaryMin != nil:
			fmt.Printf(
				"%s%d+\n",
				j.Currency,
				*j.SalaryMin,
			)
		case j.SalaryMax != nil:
			fmt.Printf(
				"Up to %s%d\n",
				j.Currency,
				*j.SalaryMax,
			)
		}

		fmt.Println()
	}

	if score == nil {
		fmt.Println("Score")
		fmt.Println("────────────────────────────")
		fmt.Println("Not scored yet")
		fmt.Println()
		return
	}

	fmt.Println("Score")
	fmt.Println("────────────────────────────")
	fmt.Printf(
		"Overall:         %.1f\n",
		score.OverallScore,
	)
	fmt.Printf(
		"Recommendation:  %s\n",
		score.Recommendation,
	)

	fmt.Println()
	fmt.Println("Score Breakdown")
	fmt.Println("────────────────────────────")

	printScore("Skills", score.SkillsScore)
	printScore("Experience", score.ExperienceScore)
	printScore("Location", score.LocationScore)
	printScore("Role", score.RoleScore)
	printScore("Domain", score.DomainScore)
	printScore("Company", score.CompanyScore)
	printScore("Compensation", score.CompensationScore)

	if score.Reasoning != "" {
		fmt.Println()
		fmt.Println("Reasoning")
		fmt.Println("────────────────────────────")
		fmt.Println(score.Reasoning)
	}

	fmt.Println()
	fmt.Println("Description")
	fmt.Println("────────────────────────────")
	fmt.Println(j.Description)
}

func printScore(name string, value *float64) {
	if value == nil {
		fmt.Printf("%-16s —\n", name)
		return
	}

	fmt.Printf("%-16s %.1f\n", name, *value)
}

func runStatusChange(
	jobID int64,
	target job.Status,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repo := job.NewSQLiteRepository(db)

	current, err := repo.GetByID(ctx, jobID)
	if err != nil {
		log.Fatalf("load job: %v", err)
	}

	if current == nil {
		log.Fatalf("job %d not found", jobID)
	}

	if current.Status == target {
		fmt.Printf(
			"Job %d is already %s\n",
			jobID,
			target,
		)
		return
	}

	if !job.CanTransition(current.Status, target) {
		log.Fatalf(
			"invalid status transition: %s -> %s",
			current.Status,
			target,
		)
	}

	if err := repo.UpdateStatus(ctx, jobID, target); err != nil {
		log.Fatalf("update job status: %v", err)
	}

	fmt.Println("JobClaw Status")
	fmt.Println("────────────────────────────")
	fmt.Printf("Job:       %d\n", jobID)
	fmt.Printf("Company:   %s\n", current.Company)
	fmt.Printf("Title:     %s\n", current.Title)
	fmt.Printf("Status:    %s → %s\n", current.Status, target)
}
