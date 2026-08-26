package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"jobclaw/internal/application"
	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/scoring"
)

// StatusReport is the machine-readable view of the whole pipeline.
//
// This exists so an orchestrator such as OpenClaw can drive JobClaw without
// scraping the human-facing output, which uses box-drawing characters and is not
// a stable contract. Field names are snake_case and the shape is additive-only:
// new fields may appear, existing ones will not change meaning.
type StatusReport struct {
	GeneratedAt string `json:"generated_at"`
	Database    string `json:"database"`

	Jobs         JobCounts         `json:"jobs"`
	Applications ApplicationCounts `json:"applications"`

	// AwaitingApproval is what a human needs to decide on: scored or
	// shortlisted jobs that have not been approved or rejected.
	AwaitingApproval []JobSummary `json:"awaiting_approval"`

	// NeedsAttention is work already approved that cannot progress on its own.
	NeedsAttention []AttentionItem `json:"needs_attention"`
}

type JobCounts struct {
	Total       int            `json:"total"`
	ByStatus    map[string]int `json:"by_status"`
	ByRecommend map[string]int `json:"by_recommendation"`
}

type ApplicationCounts struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

type JobSummary struct {
	JobID          int64    `json:"job_id"`
	Company        string   `json:"company"`
	Title          string   `json:"title"`
	Location       string   `json:"location"`
	URL            string   `json:"url"`
	Source         string   `json:"source"`
	Status         string   `json:"status"`
	Score          *float64 `json:"score,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
}

type AttentionItem struct {
	ApplicationID int64  `json:"application_id"`
	JobID         int64  `json:"job_id"`
	Company       string `json:"company"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
	NextCommand   string `json:"next_command"`
}

func runStatus(
	databasePath string,
	asJSON bool,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)
	scoreRepo := scoring.NewRepository(db)
	applicationRepo := application.NewSQLiteRepository(db)

	jobs, err := jobRepo.List(ctx, 1000)
	if err != nil {
		log.Fatalf("list jobs: %v", err)
	}

	scores, err := scoreRepo.List(ctx, 1000)
	if err != nil {
		log.Fatalf("list scores: %v", err)
	}

	applications, err := applicationRepo.List(ctx, 1000)
	if err != nil {
		log.Fatalf("list applications: %v", err)
	}

	scoreByJob := make(map[int64]scoring.Result, len(scores))

	for _, score := range scores {
		scoreByJob[score.JobID] = score
	}

	report := StatusReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Database:    databasePath,
		Jobs: JobCounts{
			Total:       len(jobs),
			ByStatus:    map[string]int{},
			ByRecommend: map[string]int{},
		},
		Applications: ApplicationCounts{
			Total:    len(applications),
			ByStatus: map[string]int{},
		},
		AwaitingApproval: []JobSummary{},
		NeedsAttention:   []AttentionItem{},
	}

	jobByID := make(map[int64]job.Job, len(jobs))

	for _, j := range jobs {
		jobByID[j.ID] = j
		report.Jobs.ByStatus[string(j.Status)]++

		score, hasScore := scoreByJob[j.ID]

		if hasScore {
			report.Jobs.ByRecommend[string(score.Recommendation)]++
		}

		// Only scored or shortlisted jobs are a decision the candidate still
		// owes. Discovered jobs are not scored yet; approved and rejected ones
		// have already been decided.
		if j.Status != job.StatusScored &&
			j.Status != job.StatusShortlisted {
			continue
		}

		summary := JobSummary{
			JobID:    j.ID,
			Company:  j.Company,
			Title:    j.Title,
			Location: j.Location,
			URL:      j.URL,
			Source:   j.Source,
			Status:   string(j.Status),
		}

		if hasScore {
			overall := score.OverallScore
			summary.Score = &overall
			summary.Recommendation = string(score.Recommendation)
		}

		report.AwaitingApproval = append(report.AwaitingApproval, summary)
	}

	for _, app := range applications {
		report.Applications.ByStatus[string(app.Status)]++

		j := jobByID[app.JobID]

		item := AttentionItem{
			ApplicationID: app.ID,
			JobID:         app.JobID,
			Company:       j.Company,
			Title:         j.Title,
			Status:        string(app.Status),
		}

		switch {
		case app.Status == application.StatusSubmissionInProgress:
			// The most important case: an unresolved external attempt that must
			// never be retried automatically.
			item.Reason = "submission outcome unconfirmed; manual reconciliation required"
			item.NextCommand = ""

		case app.TailoredResumePath == "":
			item.Reason = "tailored resume not generated"
			item.NextCommand = fmt.Sprintf("jobclaw resume %d", app.JobID)

		case app.Status == application.StatusDraft:
			item.Reason = "preparation incomplete"
			item.NextCommand = fmt.Sprintf(
				"jobclaw prepare %d",
				app.ID,
			)

		case app.Status == application.StatusReadyToApply:
			item.Reason = "ready to submit, awaiting review"
			item.NextCommand = fmt.Sprintf("jobclaw submit %d", app.ID)

		default:
			continue
		}

		report.NeedsAttention = append(report.NeedsAttention, item)
	}

	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(report); err != nil {
			log.Fatalf("encode status: %v", err)
		}

		return
	}

	printStatusReport(report)
}

func printStatusReport(report StatusReport) {
	fmt.Println("JobClaw Status")
	fmt.Println("────────────────────────────")
	fmt.Printf("Database: %s\n", report.Database)
	fmt.Println()

	fmt.Printf("Jobs (%d)\n", report.Jobs.Total)

	for _, status := range []job.Status{
		job.StatusDiscovered,
		job.StatusScoring,
		job.StatusScored,
		job.StatusShortlisted,
		job.StatusApproved,
		job.StatusRejected,
	} {
		if count := report.Jobs.ByStatus[string(status)]; count > 0 {
			fmt.Printf("  %-24s %d\n", status, count)
		}
	}

	fmt.Println()
	fmt.Printf("Applications (%d)\n", report.Applications.Total)

	for status, count := range report.Applications.ByStatus {
		fmt.Printf("  %-24s %d\n", status, count)
	}

	fmt.Println()
	fmt.Printf("Awaiting your approval (%d)\n", len(report.AwaitingApproval))

	for _, summary := range report.AwaitingApproval {
		score := "unscored"

		if summary.Score != nil {
			score = fmt.Sprintf(
				"%.1f %s",
				*summary.Score,
				summary.Recommendation,
			)
		}

		fmt.Printf(
			"  [%d] %s — %s (%s)\n",
			summary.JobID,
			summary.Company,
			summary.Title,
			score,
		)
	}

	fmt.Println()
	fmt.Printf("Needs attention (%d)\n", len(report.NeedsAttention))

	for _, item := range report.NeedsAttention {
		fmt.Printf(
			"  [app %d] %s — %s\n",
			item.ApplicationID,
			item.Company,
			item.Title,
		)
		fmt.Printf("      %s\n", item.Reason)

		if item.NextCommand != "" {
			fmt.Printf("      next: %s\n", item.NextCommand)
		}
	}
}
