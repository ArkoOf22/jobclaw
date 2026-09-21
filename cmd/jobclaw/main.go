package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jobclaw/internal/application"
	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/database"
	"jobclaw/internal/discovery"
	"jobclaw/internal/discovery/greenhouse"
	"jobclaw/internal/discovery/jobspy"
	"jobclaw/internal/drive"
	"jobclaw/internal/job"
	"jobclaw/internal/llm/openrouter"
	"jobclaw/internal/scoring"
	"jobclaw/internal/sheet"
)

const (
	defaultDatabasePath    = "data/jobclaw.db"
	defaultCandidateConfig = "config/candidate.yaml"
	defaultPreferences     = "config/preferences.yaml"
	defaultJobSpyURL       = "http://127.0.0.1:8000/mcp"

	// Greenhouse's public Job Board API. Every GET, including a job's
	// application questions, is unauthenticated; only the POST submission
	// endpoint requires a key. Defaulting this means reading an application
	// form needs no configuration at all.
	defaultGreenhouseBaseURL = "https://boards-api.greenhouse.io/v1"
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
		case "status":
			// Machine-readable pipeline state, so an orchestrator can drive
			// JobClaw without parsing the human-facing output.
			asJSON := false
			limit := defaultAwaitingApprovalLimit

			statusArgs := os.Args[2:]
			for len(statusArgs) > 0 {
				switch statusArgs[0] {
				case "--json":
					asJSON = true
					statusArgs = statusArgs[1:]

				case "--limit":
					if len(statusArgs) < 2 {
						log.Fatal("--limit requires a number (0 for no cap)")
					}

					parsed, err := strconv.Atoi(statusArgs[1])
					if err != nil || parsed < 0 {
						log.Fatalf(
							"--limit needs a non-negative number, got %q",
							statusArgs[1],
						)
					}

					limit = parsed
					statusArgs = statusArgs[2:]

				default:
					log.Fatal(
						"usage: jobclaw status [--json] [--limit N]",
					)
				}
			}

			runStatus(databasePath, asJSON, limit, db)
			return

		case "mark":
			if len(os.Args) != 4 {
				log.Fatal(
					"usage: jobclaw mark <job_id> <applied|skipped>",
				)
			}

			jobID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || jobID <= 0 {
				log.Fatal("job ID must be a positive integer")
			}

			runMark(jobID, os.Args[3], db)
			return

		case "sheet":
			if len(os.Args) < 3 {
				log.Fatal(
					"usage: jobclaw sheet <init|sync|rebuild> [--dry-run|--confirm]",
				)
			}

			switch os.Args[2] {
			case "init":
				runSheetInit(db)

			case "rebuild":
				// Empties the sheet and writes it again from current scores.
				// Destructive, so it previews unless --confirm is given, the
				// same gate as submit and prune.
				confirm := false

				if len(os.Args) == 4 {
					if os.Args[3] != "--confirm" {
						log.Fatalf(
							"unknown argument %q; usage: jobclaw sheet rebuild [--confirm]",
							os.Args[3],
						)
					}

					confirm = true
				} else if len(os.Args) > 4 {
					log.Fatal("usage: jobclaw sheet rebuild [--confirm]")
				}

				runSheetRebuild(confirm, db)

			case "sync":
				dryRun := false

				if len(os.Args) == 4 {
					if os.Args[3] != "--dry-run" {
						log.Fatalf(
							"unknown argument %q; usage: jobclaw sheet sync [--dry-run]",
							os.Args[3],
						)
					}

					dryRun = true
				} else if len(os.Args) > 4 {
					log.Fatal("usage: jobclaw sheet sync [--dry-run]")
				}

				runSheetSync(dryRun, db)

			default:
				log.Fatalf(
					"unknown sheet command %q; usage: jobclaw sheet <init|sync|rebuild>",
					os.Args[2],
				)
			}

			return

		case "prune":
			// Deletion is irreversible, so it takes the same gate as
			// submission: preview unless --confirm is given.
			olderThanDays := 30
			confirmed := false

			args := os.Args[2:]

			for len(args) > 0 {
				switch args[0] {
				case "--confirm":
					confirmed = true
					args = args[1:]

				case "--older-than":
					if len(args) < 2 {
						log.Fatal("--older-than requires a number of days")
					}

					days, err := strconv.Atoi(args[1])
					if err != nil || days < 1 {
						log.Fatal(
							"--older-than must be a positive number of days",
						)
					}

					olderThanDays = days
					args = args[2:]

				default:
					log.Fatalf(
						"unknown argument %q; usage: jobclaw prune [--older-than <days>] [--confirm]",
						args[0],
					)
				}
			}

			runPrune(olderThanDays, confirmed, db)
			return

		case "discover":
			// Defaults match the previous hardcoded values so scheduled runs
			// behave identically unless asked otherwise.
			hoursOld := 168
			limit := 10

			args := os.Args[2:]

			for len(args) > 0 {
				if len(args) < 2 {
					log.Fatalf(
						"%s requires a value; usage: jobclaw discover [--hours <n>] [--limit <n>]",
						args[0],
					)
				}

				value, err := strconv.Atoi(args[1])
				if err != nil || value < 1 {
					log.Fatalf(
						"%s must be a positive integer, got %q",
						args[0],
						args[1],
					)
				}

				switch args[0] {
				case "--hours":
					hoursOld = value

				case "--limit":
					// Per source, not in total.
					limit = value

				default:
					log.Fatalf(
						"unknown argument %q; usage: jobclaw discover [--hours <n>] [--limit <n>]",
						args[0],
					)
				}

				args = args[2:]
			}

			runDiscover(databasePath, hoursOld, limit, cfg, db)
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

		case "metrics":
			// Long-running HTTP server exposing /metrics. Address is
			// configurable so it can bind to loopback (default) or a Tailscale
			// address for off-box scraping.
			addr := getEnvOrDefault("JOBCLAW_METRICS_ADDR", "127.0.0.1:9090")

			args := os.Args[2:]
			for len(args) > 0 {
				if args[0] == "--addr" {
					if len(args) < 2 {
						log.Fatal("--addr requires a host:port value")
					}

					addr = args[1]
					args = args[2:]

					continue
				}

				log.Fatalf(
					"unknown argument %q; usage: jobclaw metrics [--addr host:port]",
					args[0],
				)
			}

			runMetrics(addr, db)
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

		case "answer":
			if len(os.Args) < 3 {
				log.Fatal("usage: jobclaw answer <add|list> ...")
			}

			switch os.Args[2] {
			case "add":
				if len(os.Args) != 5 {
					log.Fatal("usage: jobclaw answer add <field_key> <answer>")
				}

				runAnswerAdd(
					os.Args[3],
					os.Args[4],
					db,
				)
				return

			case "list":
				if len(os.Args) != 3 {
					log.Fatal("usage: jobclaw answer list")
				}

				runAnswerList(db)
				return

			case "update":
				if len(os.Args) != 5 {
					log.Fatal("usage: jobclaw answer update <field_key> <answer>")
				}

				runAnswerUpdate(
					os.Args[3],
					os.Args[4],
					db,
				)
				return

			default:
				log.Fatal("usage: jobclaw answer <add|list> ...")
			}

		case "questionnaire":
			applicationID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || applicationID <= 0 {
				log.Fatal("application ID must be a positive integer")
			}

			var (
				sourcePath     string
				fromGreenhouse bool
			)

			switch {
			case len(os.Args) == 4 && os.Args[3] == "--from-greenhouse":
				// Fetch the live application form from Greenhouse's public Job
				// Board API. Requires no credentials.
				fromGreenhouse = true

			case len(os.Args) == 5 && os.Args[3] == "--source":
				sourcePath = os.Args[4]

				if sourcePath == "" {
					log.Fatal("questionnaire source path is required")
				}

			case len(os.Args) == 3:
				// Resolve answers for questions that are already ingested.

			default:
				log.Fatal(
					"usage: jobclaw questionnaire <application_id> [--source <path> | --from-greenhouse]",
				)
			}

			runQuestionnaire(
				applicationID,
				sourcePath,
				fromGreenhouse,
				cfg,
				db,
			)

			return

		case "prepare":
			if len(os.Args) != 3 {
				log.Fatal("usage: jobclaw prepare <application_id>")
			}

			applicationID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || applicationID <= 0 {
				log.Fatal("application ID must be a positive integer")
			}

			runPrepare(applicationID, cfg, db)
			return

		case "submit":
			if len(os.Args) < 3 || len(os.Args) > 4 {
				log.Fatal(
					"usage: jobclaw submit <application_id> [--confirm]",
				)
			}

			applicationID, err := strconv.ParseInt(os.Args[2], 10, 64)
			if err != nil || applicationID <= 0 {
				log.Fatal("application ID must be a positive integer")
			}

			// Submission is irreversible and externally visible, so sending is
			// opt-in. Without --confirm this previews the payload only.
			confirmed := false

			if len(os.Args) == 4 {
				if os.Args[3] != "--confirm" {
					log.Fatalf(
						"unknown flag %q; usage: jobclaw submit <application_id> [--confirm]",
						os.Args[3],
					)
				}

				confirmed = true
			}

			runSubmit(applicationID, confirmed, db)
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
	hoursOld int,
	limit int,
	cfg *config.Config,
	db *database.DB,
) {
	// Discovery now searches each role as its own query across several boards, so
	// a run is a sequence of scrapes rather than one call. Give it room.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	repository := job.NewSQLiteRepository(db)

	jobSpyURL := getEnvOrDefault(
		"JOBCLAW_JOBSPY_URL",
		defaultJobSpyURL,
	)

	jobSpyClient := jobspy.NewClient(jobSpyURL)

	greenhouseBoards := strings.FieldsFunc(
		strings.TrimSpace(os.Getenv("JOBCLAW_GREENHOUSE_BOARDS")),
		func(r rune) bool {
			return r == ',' || r == '\n'
		},
	)

	sources := []discovery.Source{
		jobSpyClient,
	}

	if len(greenhouseBoards) > 0 {
		greenhouseClient := greenhouse.NewMultiBoardClient(
			greenhouseBoards,
		)

		sources = append(sources, greenhouseClient)
	}

	service := discovery.NewService(
		repository,
		sources...,
	)

	// Search on both primary and secondary target roles.
	//
	// Only primary was used before, so every role listed under secondary was
	// configured and silently ignored. That is why titles like "Software
	// Engineer II" were only ever matched incidentally: "Software Engineer" sits
	// in secondary. Scoring still ranks results, so widening the net here costs
	// nothing in precision.
	keywords := append(
		[]string{},
		cfg.Candidate.Candidate.TargetRoles.Primary...,
	)

	keywords = append(
		keywords,
		cfg.Candidate.Candidate.TargetRoles.Secondary...,
	)

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
		HoursOld:   hoursOld,
		Limit:      limit,
	})

	fmt.Println("JobClaw Discovery")
	fmt.Println("────────────────────────────")
	fmt.Println("Candidate:", cfg.Candidate.Candidate.Name)
	fmt.Println("Database:", databasePath)
	fmt.Printf("Window:    posted within %dh\n", hoursOld)
	fmt.Printf("Limit:     %d per source\n", limit)
	fmt.Println()

	for _, result := range results {
		fmt.Printf(
			"Source: %s\nFetched: %d\nStored: %d\nDuration: %s\n",
			result.Source,
			result.Fetched,
			result.Stored,
			result.Duration.Round(time.Millisecond),
		)

		switch {
		case result.Partial():
			// Distinguish "some of this source is broken" from "this source
			// produced nothing". A flat FAILED next to a non-zero Stored count
			// reads as a contradiction and invites the wrong conclusion.
			fmt.Println("Status: PARTIAL")
			fmt.Println("Error:", result.Error)

		case result.Failed:
			fmt.Println("Status: FAILED")
			fmt.Println("Error:", result.Error)

		default:
			fmt.Println("Status: OK")
		}

		fmt.Println()
	}
}

func runJobsList(db *database.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := job.NewSQLiteRepository(db)

	const listLimit = 20

	jobs, err := repository.List(ctx, listLimit)
	if err != nil {
		log.Fatalf("list jobs: %v", err)
	}

	fmt.Println("JobClaw Jobs")
	fmt.Println("────────────────────────────")

	// Report this as a page, not a total. The previous wording, "Stored jobs:
	// %d" against len(jobs), read as the whole table while only ever showing the
	// first 20, which made a database of several hundred look like twenty.
	// Use `jobclaw status` for real counts.
	fmt.Printf("Showing up to %d jobs\n", listLimit)

	if len(jobs) == listLimit {
		fmt.Println("There may be more; run `jobclaw status` for totals.")
	}

	fmt.Println()

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
	// LLM tailoring, then two pdflatex passes, then an upload. Wider than the
	// old text-only path.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)

	resumeConfig := cfg.Resume.Resume
	candidate := cfg.Candidate.Candidate

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

	llmClient, err := openrouter.NewClient(
		openrouter.Config{
			APIKey:                   apiKey,
			Model:                    resumeConfig.LLM.Model,
			BaseURL:                  resumeConfig.LLM.BaseURL,
			DenyDataCollection:       resumeConfig.LLM.Privacy.DenyDataCollection,
			RequireZeroDataRetention: resumeConfig.LLM.Privacy.RequireZeroDataRetention,
		},
	)
	if err != nil {
		log.Fatalf("initialize resume LLM: %v", err)
	}

	compiler := application.NewPdfLatexCompiler(
		getEnvOrDefault("JOBCLAW_PDFLATEX_BIN", "pdflatex"),
	)

	generator := application.NewLaTeXResumeGenerator(
		jobRepo,
		resumeSource,
		llmClient,
		compiler,
		resumeContactFromConfig(candidate),
		resumeEducationFromConfig(candidate),
	)

	workDir := filepath.Join(
		"data",
		"applications",
		strconv.FormatInt(jobID, 10),
		"resume",
	)

	artifacts, err := generator.GenerateResumePDF(ctx, jobID, workDir)
	if err != nil {
		log.Fatalf("generate tailored resume: %v", err)
	}

	fmt.Println("JobClaw Resume")
	fmt.Println("────────────────────────────")
	fmt.Printf("Job ID:     %d\n", jobID)
	fmt.Printf("Model:      %s\n", resumeConfig.LLM.Model)
	fmt.Printf("PDF:        %s\n", artifacts.PDFPath)
	fmt.Printf("Text:       %s\n", artifacts.TextPath)

	// Upload the PDF to Drive when configured, so the candidate can open it from
	// their phone straight off the sheet. Best-effort: the PDF stands on disk
	// regardless, and the sheet falls back to the local path.
	resumeLink := artifacts.PDFPath

	if link := uploadResumeToDrive(
		ctx,
		jobRepo,
		jobID,
		candidate.Name,
		artifacts.PDFPath,
	); link != "" {
		resumeLink = link
		fmt.Printf("Drive:      %s\n", link)
	}

	updateSheetStatus(
		context.Background(),
		jobID,
		sheet.RowUpdate{
			Status:     sheetStatusResumeReady,
			ResumeLink: resumeLink,
		},
	)
}

// resumeContactFromConfig maps the candidate's config identity into the render
// contact. These are facts the model never sees.
func resumeContactFromConfig(c config.Candidate) application.ResumeContact {
	return application.ResumeContact{
		Name:     c.Name,
		Phone:    c.Contact.Phone,
		Email:    c.Contact.Email,
		LinkedIn: c.Contact.LinkedIn,
		GitHub:   c.Contact.GitHub,
		LeetCode: c.Contact.LeetCode,
	}
}

// resumeEducationFromConfig builds the education section from config. One entry
// today; a slice so more can be added without a signature change.
func resumeEducationFromConfig(c config.Candidate) []application.ResumeEducation {
	e := c.Education

	if strings.TrimSpace(e.Institution) == "" {
		return nil
	}

	dates := ""
	if e.GraduationYear > 0 {
		dates = strconv.Itoa(e.GraduationYear)
	}

	degree := e.Degree
	if e.CGPA > 0 {
		degree = fmt.Sprintf("%s  CGPA: %.2f", e.Degree, e.CGPA)
	}

	return []application.ResumeEducation{
		{
			Institution:  e.Institution,
			Dates:        dates,
			DegreeAndGPA: degree,
		},
	}
}

// uploadResumeToDrive uploads the compiled PDF and returns a viewable link, or
// an empty string if upload is unconfigured or fails. Never fatal: a resume that
// exists locally is still usable.
func uploadResumeToDrive(
	ctx context.Context,
	jobRepo *job.SQLiteRepository,
	jobID int64,
	candidateName string,
	pdfPath string,
) string {
	uploader := drive.NewUploader(drive.Config{
		Account:        strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ACCOUNT")),
		Binary:         getEnvOrDefault("JOBCLAW_GOG_BIN", "gog"),
		ParentFolderID: strings.TrimSpace(os.Getenv("JOBCLAW_RESUME_DRIVE_FOLDER")),
	})

	result, err := uploader.Upload(ctx, pdfPath, resumeFileName(
		ctx,
		jobRepo,
		jobID,
		candidateName,
	))
	if err != nil {
		fmt.Printf("Drive:      not uploaded (%v)\n", err)

		return ""
	}

	return result.Link
}

// resumeFileName builds a human-friendly Drive filename, using the company when
// the job can be loaded and falling back to the job ID otherwise.
func resumeFileName(
	ctx context.Context,
	jobRepo *job.SQLiteRepository,
	jobID int64,
	candidateName string,
) string {
	name := strings.TrimSpace(candidateName)
	if name == "" {
		name = "Resume"
	}

	if j, err := jobRepo.GetByID(ctx, jobID); err == nil && j != nil {
		company := strings.TrimSpace(j.Company)
		if company != "" {
			return fmt.Sprintf("%s - %s.pdf", name, company)
		}
	}

	return fmt.Sprintf("%s - job %d.pdf", name, jobID)
}

func runPrepare(
	applicationID int64,
	cfg *config.Config,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	applicationRepo := application.NewSQLiteRepository(db)
	questionRepo := application.NewSQLiteQuestionRepository(db)
	answerRepo := application.NewSQLiteAnswerRepository(db)
	eventRepo := application.NewSQLiteEventRepository(db)

	answerResolver := application.NewAnswerResolver(answerRepo)

	// Preparation validates; it does not generate. Questions that already carry
	// an answer are left untouched by the questionnaire service, so this needs
	// no LLM and makes no network calls. Unresolved questions are answered by
	// `jobclaw questionnaire`.
	questionnaireService := application.NewQuestionnaireService(
		questionRepo,
		answerResolver,
		nil,
		"",
		eventRepo,
	)

	service := application.NewApplicationPreparationService(
		applicationRepo,
		questionRepo,
		eventRepo,
		questionnaireService,
	)

	readiness, err := service.Prepare(
		ctx,
		applicationID,
	)
	if err != nil {
		log.Fatalf("prepare application: %v", err)
	}

	fmt.Println("JobClaw Application Preparation")
	fmt.Println("────────────────────────────")
	fmt.Printf("Application ID: %d\n", applicationID)
	fmt.Printf("Readiness:      %s\n", readiness.Status)
	fmt.Println()

	for _, check := range readiness.Checks {
		switch check.Status {
		case application.ReadinessReady:
			fmt.Printf("✓ %-20s %s\n", check.Name, check.Reason)

		case application.ReadinessBlocked:
			fmt.Printf("⚠ %-20s %s\n", check.Name, check.Reason)
		}
	}

	if !readiness.Ready() {
		fmt.Println()
		fmt.Println("Application is NOT ready to apply.")
		fmt.Println()
		fmt.Println("Blockers:")

		for _, blocker := range readiness.Blockers {
			fmt.Printf("  • %s\n", blocker)
		}

		return
	}

	fmt.Println()
	fmt.Println("✓ Application is READY_TO_APPLY")
}

func runSubmit(
	applicationID int64,
	confirmed bool,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	applicationRepo := application.NewSQLiteRepository(db)
	jobRepo := job.NewSQLiteRepository(db)
	eventRepo := application.NewSQLiteEventRepository(db)
	questionRepo := application.NewSQLiteQuestionRepository(db)
	answerRepo := application.NewSQLiteAnswerRepository(db)

	registry := application.NewStaticSubmissionAdapterRegistry()

	// Tracked separately so the dry-run preview can report which targets are
	// actually wired. The registry exposes no listing of its own.
	var targets []application.SubmissionTargetType

	manualAdapter := application.NewManualSubmissionAdapter()
	if err := registry.Register(
		application.SubmissionTargetManual,
		manualAdapter,
	); err != nil {
		fmt.Printf("configure submission adapter: %v\n", err)
		return
	}

	targets = append(targets, application.SubmissionTargetManual)

	// Register the Greenhouse adapter only when a Job Board API Key is present,
	// since only the POST submission endpoint requires one. Gating on the base
	// URL instead used to abort the entire command, including the dry run, when
	// a URL was configured without a key.
	//
	// Note that the Job Board API Key is issued by the employer from their own
	// Greenhouse settings. An applicant cannot obtain one for a company they do
	// not work for, so automated submission is generally unavailable and the
	// MANUAL target is the realistic terminal step.
	if greenhouseAPIKey := strings.TrimSpace(
		os.Getenv("JOBCLAW_GREENHOUSE_API_KEY"),
	); greenhouseAPIKey != "" {
		greenhouseBaseURL := getEnvOrDefault(
			"JOBCLAW_GREENHOUSE_BASE_URL",
			defaultGreenhouseBaseURL,
		)

		httpClient := &http.Client{
			Timeout: 20 * time.Second,
		}

		greenhouseAdapter := application.NewGreenhouseSubmissionAdapter(
			httpClient,
			greenhouseBaseURL,
		)

		greenhouseAdapter.SetAPIKey(
			greenhouseAPIKey,
		)

		greenhouseAdapter.SetFormProvider(
			application.NewGreenhouseHTTPFormProvider(
				httpClient,
				greenhouseBaseURL,
			),
		)

		if err := registry.Register(
			application.SubmissionTargetGreenhouse,
			greenhouseAdapter,
		); err != nil {
			fmt.Printf("configure greenhouse submission adapter: %v\n", err)
			return
		}

		targets = append(targets, application.SubmissionTargetGreenhouse)
	}

	submitter := application.NewRegistrySubmissionSubmitter(
		registry,
	)
	submitter.SetDataProvider(
		questionRepo,
		application.NewAnswerResolver(answerRepo),
	)

	transactionFactory := application.NewSQLiteSubmissionTransactionFactory(db)

	service := application.NewApplicationSubmissionService(
		applicationRepo,
		jobRepo,
		eventRepo,
		submitter,
	)
	service.SetTransactionFactory(transactionFactory)

	fmt.Println("JobClaw Application Submission")
	fmt.Println("────────────────────────────")
	fmt.Printf("Application ID: %d\n", applicationID)
	fmt.Println()

	// Submitting is irreversible and externally visible. Preview is the
	// default; sending requires an explicit --confirm.
	if !confirmed {
		runSubmissionPreview(
			ctx,
			applicationID,
			applicationRepo,
			jobRepo,
			questionRepo,
			targets,
		)

		return
	}

	err := service.Submit(
		ctx,
		applicationID,
	)

	// Ambiguous is not failure. The request may have reached the employer, so
	// this must never be reported as "nothing was submitted": that invites a
	// resubmission and a duplicate application.
	if errors.Is(err, application.ErrSubmissionAmbiguous) {
		fmt.Println("Outcome: AMBIGUOUS — DO NOT RESUBMIT")
		fmt.Println()
		fmt.Println("The request may have reached the employer. JobClaw cannot")
		fmt.Println("confirm whether the application was accepted, so it will")
		fmt.Println("not retry automatically.")
		fmt.Println()
		fmt.Println("The application stays locked in SUBMISSION_IN_PROGRESS")
		fmt.Println("until you reconcile it manually. Check the employer's")
		fmt.Println("careers portal or your email for a confirmation before")
		fmt.Println("taking any further action.")
		fmt.Println()
		fmt.Printf("Details: %v\n", err)

		os.Exit(2)
	}

	if err != nil {
		fmt.Println("Outcome: NOT SUBMITTED")
		fmt.Println()
		fmt.Println("The application was not sent and local state is unchanged.")
		fmt.Println()
		fmt.Printf("Details: %v\n", err)

		os.Exit(1)
	}

	fmt.Println("Outcome: SUBMITTED")
	fmt.Println("The employer confirmed receipt of the application.")
}

// runSubmissionPreview shows what would be sent without crossing the network.
// Prepared data is assembled through the same path a real submission uses, so
// the preview reflects the actual payload rather than a reconstruction.
func runSubmissionPreview(
	ctx context.Context,
	applicationID int64,
	applicationRepo *application.SQLiteRepository,
	jobRepo *job.SQLiteRepository,
	questionRepo application.QuestionRepository,
	targets []application.SubmissionTargetType,
) {
	fmt.Println("Mode: DRY RUN — nothing will be sent")
	fmt.Println()

	app, err := applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		fmt.Printf("load application: %v\n", err)
		os.Exit(1)
	}

	if app == nil {
		fmt.Printf("application %d not found\n", applicationID)
		os.Exit(1)
	}

	j, err := jobRepo.GetByID(ctx, app.JobID)
	if err != nil {
		fmt.Printf("load job: %v\n", err)
		os.Exit(1)
	}

	if j == nil {
		fmt.Printf("job %d not found\n", app.JobID)
		os.Exit(1)
	}

	fmt.Println("Target")
	fmt.Printf("  Company:  %s\n", j.Company)
	fmt.Printf("  Role:     %s\n", j.Title)
	fmt.Printf("  Location: %s\n", j.Location)
	fmt.Printf("  URL:      %s\n", j.URL)
	fmt.Println()

	fmt.Println("Application state")
	fmt.Printf("  Status:   %s\n", app.Status)

	if app.TailoredResumePath == "" {
		fmt.Println("  Resume:   MISSING")
	} else {
		fmt.Printf("  Resume:   %s\n", app.TailoredResumePath)
	}

	fmt.Println()

	questions, err := questionRepo.ListByApplicationID(ctx, applicationID)
	if err != nil {
		fmt.Printf("load questions: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Questionnaire (%d question(s))\n", len(questions))

	if len(questions) == 0 {
		fmt.Println("  none ingested")
	}

	unanswered := 0

	for _, question := range questions {
		answer := question.Answer

		if strings.TrimSpace(answer) == "" {
			answer = "UNANSWERED"
			unanswered++
		}

		fmt.Printf("  • %s\n", question.Question)
		fmt.Printf(
			"      field=%s status=%s source=%s\n",
			question.FieldKey,
			question.Status,
			question.AnswerSource,
		)
		fmt.Printf("      answer=%s\n", answer)
	}

	if unanswered > 0 {
		fmt.Println()
		fmt.Printf(
			"  %d question(s) have no answer and would be sent blank.\n",
			unanswered,
		)
	}

	fmt.Println()
	fmt.Println("Configured submission targets")

	if len(targets) == 0 {
		fmt.Println("  none")
	}

	for _, target := range targets {
		fmt.Printf("  • %s\n", target)
	}

	fmt.Println()
	fmt.Println("────────────────────────────")
	fmt.Println("Nothing was sent. Review the payload above, then run:")
	fmt.Printf("  jobclaw submit %d --confirm\n", applicationID)
}

func runApplication(jobID int64, cfg *config.Config, db *database.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)
	applicationRepo := application.NewSQLiteRepository(db)
	eventRepo := application.NewSQLiteEventRepository(db)

	resumeConfig := cfg.Resume.Resume

	// The resume generator is optional here. Creating the application and its
	// workspace is a local operation, so it must not require the LLM to be
	// configured or reachable. When the key is absent the application is still
	// created and the resume step is reported as pending, retryable with
	// `jobclaw resume <id>`.
	var resumeGenerator application.ResumeGenerator

	apiKey := os.Getenv(resumeConfig.LLM.APIKeyEnv)

	if apiKey != "" {
		resumeGenerator = buildResumeGenerator(
			jobRepo,
			resumeConfig,
			apiKey,
		)
	}

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
	fmt.Println()

	if apiKey == "" {
		fmt.Println("Tailored resume: PENDING")
		fmt.Printf(
			"  %s is not set, so the resume was not generated.\n",
			resumeConfig.LLM.APIKeyEnv,
		)
		fmt.Printf(
			"  The application is intact. Retry with: jobclaw resume %d\n",
			jobID,
		)

		return
	}

	resumePath, err := service.EnsureTailoredResume(ctx, jobID)
	if err != nil {
		// The application row is already committed and valid. A resume failure
		// is recoverable, so report it without discarding that work.
		fmt.Println("Tailored resume: FAILED")
		fmt.Printf("  %v\n", err)
		fmt.Printf(
			"  The application is intact. Retry with: jobclaw resume %d\n",
			jobID,
		)

		os.Exit(1)
	}

	fmt.Println("Tailored resume: OK")
	fmt.Printf("  %s\n", resumePath)
}

// buildResumeGenerator assembles the LLM-backed resume generator. Shared by the
// application, resume, and questionnaire commands, which previously duplicated
// this wiring three times.
func buildResumeGenerator(
	jobRepo *job.SQLiteRepository,
	resumeConfig config.Resume,
	apiKey string,
) application.ResumeGenerator {
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
			DenyDataCollection: resumeConfig.LLM.Privacy.
				DenyDataCollection,
			RequireZeroDataRetention: resumeConfig.LLM.Privacy.
				RequireZeroDataRetention,
		},
	)
	if err != nil {
		log.Fatalf("initialize resume LLM: %v", err)
	}

	return application.NewLLMResumeGenerator(
		jobRepo,
		promptBuilder,
		llmClient,
	)
}

func runAnswerAdd(
	fieldKey string,
	answer string,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	repo := application.NewSQLiteAnswerRepository(db)

	candidateAnswer := application.CandidateAnswer{
		FieldKey:  fieldKey,
		Answer:    answer,
		ValueType: application.AnswerValueText,
		Verified:  true,
	}

	if err := repo.Create(ctx, candidateAnswer); err != nil {
		log.Fatalf("add candidate answer: %v", err)
	}

	fmt.Println("Candidate answer added")
	fmt.Println("────────────────────────────")
	fmt.Printf("Field:    %s\n", fieldKey)
	fmt.Printf("Answer:   %s\n", answer)
	fmt.Printf("Verified: true\n")
}

func runAnswerUpdate(
	fieldKey string,
	answer string,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	repo := application.NewSQLiteAnswerRepository(db)

	existing, err := repo.GetByFieldKey(ctx, fieldKey)
	if err != nil {
		log.Fatalf("find candidate answer: %v", err)
	}

	if existing == nil {
		log.Fatalf("candidate answer for field %q does not exist", fieldKey)
	}

	existing.Answer = answer
	existing.Verified = true

	if err := repo.Update(ctx, *existing); err != nil {
		log.Fatalf("update candidate answer: %v", err)
	}

	fmt.Println("Candidate answer updated")
	fmt.Println("────────────────────────────")
	fmt.Printf("Field:    %s\n", fieldKey)
	fmt.Printf("Answer:   %s\n", answer)
	fmt.Printf("Verified: true\n")
}

func runAnswerList(db *database.DB) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	repo := application.NewSQLiteAnswerRepository(db)

	answers, err := repo.List(ctx)
	if err != nil {
		log.Fatalf("list candidate answers: %v", err)
	}

	fmt.Println("JobClaw Candidate Answer Bank")
	fmt.Println("────────────────────────────")

	if len(answers) == 0 {
		fmt.Println("No candidate answers configured.")
		return
	}

	for _, answer := range answers {
		status := "UNVERIFIED"
		if answer.Verified {
			status = "VERIFIED"
		}

		fmt.Printf(
			"[%d] %-24s %s (%s)\n",
			answer.ID,
			answer.FieldKey,
			answer.Answer,
			status,
		)
	}

	fmt.Println()
	fmt.Printf("Total answers: %d\n", len(answers))
}

func runQuestionnaire(
	applicationID int64,
	sourcePath string,
	fromGreenhouse bool,
	cfg *config.Config,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	if cfg == nil {
		log.Fatal("configuration is required")
	}

	questionRepo := application.NewSQLiteQuestionRepository(db)
	answerRepo := application.NewSQLiteAnswerRepository(db)
	eventRepo := application.NewSQLiteEventRepository(db)

	resolver := application.NewAnswerResolver(answerRepo)

	resumeConfig := cfg.Resume.Resume

	apiKey := os.Getenv(resumeConfig.LLM.APIKeyEnv)
	if apiKey == "" {
		log.Fatalf(
			"questionnaire LLM API key environment variable %q is not set",
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

	contextBuilder := application.NewCandidateContextBuilder(
		cfg.Candidate.Candidate,
		resumeSource,
	)

	candidateContext, err := contextBuilder.Build()
	if err != nil {
		log.Fatalf("build candidate context: %v", err)
	}

	llmClient, err := openrouter.NewClient(
		openrouter.Config{
			APIKey:  apiKey,
			Model:   resumeConfig.LLM.Model,
			BaseURL: resumeConfig.LLM.BaseURL,
			DenyDataCollection: resumeConfig.LLM.Privacy.
				DenyDataCollection,
			RequireZeroDataRetention: resumeConfig.LLM.Privacy.
				RequireZeroDataRetention,
		},
	)
	if err != nil {
		log.Fatalf("initialize questionnaire LLM: %v", err)
	}

	questionAnswerLLM := application.NewLLMQuestionAnswerGenerator(
		llmClient,
	)

	var results []application.AnswerResolution

	if fromGreenhouse {
		results = ingestGreenhouseQuestionnaire(
			ctx,
			applicationID,
			db,
			questionRepo,
			eventRepo,
			resolver,
			questionAnswerLLM,
			candidateContext,
		)
	} else if sourcePath != "" {
		raw, err := os.ReadFile(filepath.Clean(sourcePath))
		if err != nil {
			log.Fatalf(
				"read questionnaire source %q: %v",
				sourcePath,
				err,
			)
		}

		ingestor := application.NewQuestionnaireIngestor(
			questionRepo,
			eventRepo,
		)

		extractor := application.NewTextQuestionnaireExtractor()

		service := application.NewApplicationQuestionnaireService(
			extractor,
			ingestor,
			questionRepo,
			resolver,
			questionAnswerLLM,
			candidateContext,
			eventRepo,
		)

		results, err = service.ProcessSource(
			ctx,
			applicationID,
			string(raw),
		)
		if err != nil {
			log.Fatalf("process questionnaire source: %v", err)
		}
	} else {
		service := application.NewQuestionnaireService(
			questionRepo,
			resolver,
			questionAnswerLLM,
			candidateContext,
			eventRepo,
		)

		results, err = service.ProcessApplication(
			ctx,
			applicationID,
		)
		if err != nil {
			log.Fatalf("process questionnaire: %v", err)
		}
	}

	fmt.Println("JobClaw Questionnaire")
	fmt.Println("────────────────────────────")
	fmt.Printf("Application ID: %d\n", applicationID)

	if sourcePath != "" {
		fmt.Printf("Source:         %s\n", sourcePath)
	}

	fmt.Println()

	answered := 0
	needsReview := 0

	for _, result := range results {
		switch result.Status {
		case application.ResolutionAnswered:
			answered++

			fmt.Printf(
				"✓ [%d] %s → %s",
				result.QuestionID,
				result.FieldKey,
				result.Answer,
			)

			if result.Source == application.AnswerSourceLLM {
				fmt.Print(" (LLM)")
			}

			fmt.Println()

		case application.ResolutionNeedsReview:
			needsReview++

			fmt.Printf(
				"⚠ [%d] NEEDS_REVIEW → %s\n",
				result.QuestionID,
				result.Reason,
			)
		}
	}

	fmt.Println()
	fmt.Printf("Questions:    %d\n", len(results))
	fmt.Printf("Answered:     %d\n", answered)
	fmt.Printf("Needs Review: %d\n", needsReview)
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
	// Generous, because this now covers the whole table rather than a window of
	// 1000, and each job may trigger an on-demand company classification. A
	// deadline hit mid-run leaves the table half re-scored under old rules and
	// half under new, which is worse than a slow run.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)
	companyRepo := company.NewRepository(db)
	scoreRepo := scoring.NewRepository(db)

	scorer := scoring.NewScorer(
		cfg.Candidate.Candidate,
		cfg.Preferences.JobPreferences,
	)

	// Company classification is a scoring input, and under
	// require_product_company an UNKNOWN classification forces SKIP. Wire the
	// classifier in so scoring can classify on demand instead of depending on
	// a separate step that nothing in this CLI ever ran.
	companyService := company.NewService(
		companyRepo,
		company.NewEvidenceCollector(db),
		company.NewEvidenceClassifier(
			company.NewRuleBasedClassifier(),
		),
	)

	service := scoring.NewService(
		jobRepo,
		companyRepo,
		scoreRepo,
		scorer,
	).WithCompanyClassifier(companyService)

	// Score every job, not a fixed newest-N window.
	//
	// The cap used to be 1000 against a growing table, justified by the newest
	// jobs always being covered and the overflow being "old and already
	// scored". That reasoning only holds while the rules do not change. When the
	// experience filter was corrected, 7 of the 11 postings it should have
	// vetoed sat outside the window and kept their old SHORTLIST, so the fix
	// could not reach the jobs the candidate was actually looking at.
	total, err := jobRepo.Count(ctx)
	if err != nil {
		log.Fatalf("count jobs: %v", err)
	}

	jobs, err := jobRepo.List(ctx, total)
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

	// Jobs the candidate has already acted on, or that have moved past the
	// review stage. The shortlist is a worklist, so a job stays on it only while
	// there is still a decision to make about it. Scoring does not know any of
	// this: a job keeps its SHORTLIST recommendation forever, which is why the
	// filter lives here and not in the query.
	settled := map[job.Status]bool{
		job.StatusApplied:   true,
		job.StatusRejected:  true,
		job.StatusInterview: true,
		job.StatusOffer:     true,
	}

	skipped := 0

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

		if settled[j.Status] {
			skipped++

			continue
		}

		count++

		fmt.Printf(
			"[job %d]  %s — %s\n",
			j.ID,
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
	fmt.Printf("Awaiting your decision: %d\n", count)

	if skipped > 0 {
		fmt.Printf("Already actioned (hidden): %d\n", skipped)
	}

	if count == 0 {
		fmt.Println()
		fmt.Println("Nothing to review. Discovery runs at 00/06/12/18 IST.")
	}

	if count > 0 {
		fmt.Println()
		fmt.Println("Use the [job N] number with other commands, e.g.:")
		fmt.Println("  jobclaw mark <N> applied     jobclaw mark <N> skipped")
	}
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

func parseCommaSeparatedEnv(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		values = append(values, part)
	}

	return values
}

// ingestGreenhouseQuestionnaire fetches an application form from Greenhouse's
// public Job Board API, ingests its questions, and resolves the answers.
//
// This closes the last gap in the pipeline. The HTTP form provider and the
// questionnaire ingestor both already existed, but nothing connected them, so
// questions could only be ingested from a hand-written local text file. The form
// provider was wired solely into the submission adapter, which needs a Job Board
// API Key that only the employer can issue.
//
// Reading the form needs no credentials: every GET on the Job Board API is
// public. See https://docs.greenhouse.io/job-board.html
func ingestGreenhouseQuestionnaire(
	ctx context.Context,
	applicationID int64,
	db *database.DB,
	questionRepo application.QuestionRepository,
	eventRepo application.EventRepository,
	resolver *application.AnswerResolver,
	questionAnswerLLM application.QuestionAnswerLLM,
	candidateContext string,
) []application.AnswerResolution {
	applicationRepo := application.NewSQLiteRepository(db)
	jobRepo := job.NewSQLiteRepository(db)

	app, err := applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		log.Fatalf("load application: %v", err)
	}

	if app == nil {
		log.Fatalf("application %d not found", applicationID)
	}

	j, err := jobRepo.GetByID(ctx, app.JobID)
	if err != nil {
		log.Fatalf("load job: %v", err)
	}

	if j == nil {
		log.Fatalf("job %d not found", app.JobID)
	}

	baseURL := getEnvOrDefault(
		"JOBCLAW_GREENHOUSE_BASE_URL",
		defaultGreenhouseBaseURL,
	)

	provider := application.NewGreenhouseHTTPFormProvider(
		&http.Client{Timeout: 30 * time.Second},
		baseURL,
	)

	// Prefer the token recorded at discovery time. Employers commonly host their
	// Greenhouse board on their own domain, where the token is absent from the
	// job URL entirely, so it cannot be re-derived.
	switch {
	case j.BoardToken != "":
		provider.SetBoardToken(j.BoardToken)

	default:
		// Jobs discovered before the token was persisted have none recorded.
		// Fall back to the configured discovery boards, but only when there is
		// exactly one, since more than one is ambiguous.
		boards := parseCommaSeparatedEnv("JOBCLAW_GREENHOUSE_BOARDS")

		if len(boards) == 1 {
			provider.SetBoardToken(boards[0])

			fmt.Printf(
				"note: job %d has no recorded board token; assuming %q from JOBCLAW_GREENHOUSE_BOARDS\n",
				j.ID,
				boards[0],
			)
		}
	}

	form, err := provider.GetApplicationForm(ctx, *j)
	if err != nil {
		log.Fatalf("fetch greenhouse application form: %v", err)
	}

	inputs := application.GreenhouseFormToQuestionnaireInputs(form)

	if len(inputs) == 0 {
		log.Fatalf(
			"greenhouse form for job %d produced no questions; refusing to record an empty questionnaire",
			j.ID,
		)
	}

	fmt.Println("JobClaw Questionnaire")
	fmt.Println("────────────────────────────")
	fmt.Printf("Source:      greenhouse (%s)\n", baseURL)
	fmt.Printf("Job:         %s — %s\n", j.Company, j.Title)
	fmt.Printf("Form fields: %d\n", len(form.Fields))
	fmt.Printf("Questions:   %d after dropping artifact fields\n", len(inputs))
	fmt.Println()

	ingestor := application.NewQuestionnaireIngestor(
		questionRepo,
		eventRepo,
	)

	if err := ingestor.Ingest(ctx, applicationID, inputs); err != nil {
		log.Fatalf("ingest greenhouse questionnaire: %v", err)
	}

	service := application.NewQuestionnaireService(
		questionRepo,
		resolver,
		questionAnswerLLM,
		candidateContext,
		eventRepo,
	)

	results, err := service.ProcessApplication(ctx, applicationID)
	if err != nil {
		log.Fatalf("resolve greenhouse questionnaire: %v", err)
	}

	return results
}

// buildQuestionAnswerLLM assembles the questionnaire answer generator and the
// candidate context it draws on. Shared by the questionnaire and prepare
// commands so both resolve with identical capability.
func buildQuestionAnswerLLM(
	cfg *config.Config,
	apiKey string,
) (application.QuestionAnswerLLM, string) {
	resumeConfig := cfg.Resume.Resume

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

	contextBuilder := application.NewCandidateContextBuilder(
		cfg.Candidate.Candidate,
		resumeSource,
	)

	candidateContext, err := contextBuilder.Build()
	if err != nil {
		log.Fatalf("build candidate context: %v", err)
	}

	// Questionnaire answers are short factual strings, so they use the cheaper
	// answers model. Privacy constraints are identical: these answers are still
	// personal data about the candidate.
	llmClient, err := openrouter.NewClient(
		openrouter.Config{
			APIKey:  apiKey,
			Model:   resumeConfig.LLM.AnswersModelOrDefault(),
			BaseURL: resumeConfig.LLM.BaseURL,
			DenyDataCollection: resumeConfig.LLM.Privacy.
				DenyDataCollection,
			RequireZeroDataRetention: resumeConfig.LLM.Privacy.
				RequireZeroDataRetention,
		},
	)
	if err != nil {
		log.Fatalf("initialize questionnaire LLM: %v", err)
	}

	return application.NewLLMQuestionAnswerGenerator(llmClient), candidateContext
}
