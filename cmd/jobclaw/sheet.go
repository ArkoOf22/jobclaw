package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/sheet"
)

// runSheetSync appends newly shortlisted jobs to the Google Sheet review list.
//
// The sheet is a pure output: JobClaw keeps the truth in SQLite and records what
// it has already written, so this is safe to run on a timer and will not
// duplicate rows.
//
// Resume generation is deliberately not triggered here. Each resume is a paid LLM
// call, so they are produced on request for the specific jobs worth applying to,
// not for everything that lands on the sheet.
func runSheetSync(
	dryRun bool,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	spreadsheetID := strings.TrimSpace(
		os.Getenv("JOBCLAW_SHEET_ID"),
	)

	if spreadsheetID == "" {
		log.Fatal(
			"JOBCLAW_SHEET_ID is not set; create a sheet and put its ID in .env",
		)
	}

	writer, err := sheet.NewGogWriter(sheet.Config{
		SpreadsheetID: spreadsheetID,
		SheetName: getEnvOrDefault(
			"JOBCLAW_SHEET_TAB",
			"Jobs",
		),
		Account: strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ACCOUNT")),
		Binary:  getEnvOrDefault("JOBCLAW_GOG_BIN", "gog"),
		DryRun:  dryRun,
	})
	if err != nil {
		log.Fatalf("configure sheet writer: %v", err)
	}

	jobRepo := job.NewSQLiteRepository(db)

	candidates, err := jobRepo.ListUnsyncedForSheet(ctx, 200)
	if err != nil {
		log.Fatalf("list jobs for sheet: %v", err)
	}

	fmt.Println("JobClaw Sheet Sync")
	fmt.Println("────────────────────────────")
	fmt.Printf("Spreadsheet: %s\n", spreadsheetID)
	fmt.Printf("New rows:    %d\n", len(candidates))

	if dryRun {
		fmt.Println("Mode:        DRY RUN")
	}

	fmt.Println()

	if len(candidates) == 0 {
		fmt.Println("Nothing new to add.")

		return
	}

	rows := make([]sheet.Row, 0, len(candidates))
	ids := make([]int64, 0, len(candidates))

	for _, candidate := range candidates {
		rows = append(rows, sheet.Row{
			JobID:          candidate.Job.ID,
			Score:          fmt.Sprintf("%.1f", candidate.OverallScore),
			Recommendation: candidate.Recommendation,
			Company:        candidate.Job.Company,
			Title:          candidate.Job.Title,
			Location:       candidate.Job.Location,
			Source:         candidate.Job.Source,
			ApplyURL:       candidate.Job.URL,
			// Filled in later, when a resume is requested for this job.
			ResumeLink:   "",
			Status:       "NEW",
			Reasoning:    shortenReasoning(candidate.Reasoning),
			DiscoveredAt: candidate.Job.DiscoveredAt,
		})

		ids = append(ids, candidate.Job.ID)

		fmt.Printf(
			"  [%d] %-22s %-42s %5.1f %s\n",
			candidate.Job.ID,
			truncate(candidate.Job.Company, 22),
			truncate(candidate.Job.Title, 42),
			candidate.OverallScore,
			candidate.Recommendation,
		)
	}

	fmt.Println()

	if err := writer.Append(ctx, rows); err != nil {
		log.Fatalf("append to sheet: %v", err)
	}

	if dryRun {
		fmt.Println("Dry run: nothing was written and nothing was marked synced.")

		return
	}

	// Marked only after a successful write, so a failure leaves these eligible
	// for the next run instead of losing them.
	if err := jobRepo.MarkSheetSynced(ctx, ids); err != nil {
		log.Fatalf(
			"rows were written but marking them synced failed, which risks duplicates on the next run: %v",
			err,
		)
	}

	fmt.Printf("Appended %d row(s).\n", len(rows))
}

// runSheetInit writes the header row into a new sheet.
func runSheetInit(db *database.DB) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Minute,
	)
	defer cancel()

	spreadsheetID := strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ID"))

	if spreadsheetID == "" {
		log.Fatal("JOBCLAW_SHEET_ID is not set")
	}

	writer, err := sheet.NewGogWriter(sheet.Config{
		SpreadsheetID: spreadsheetID,
		SheetName:     getEnvOrDefault("JOBCLAW_SHEET_TAB", "Jobs"),
		Account:       strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ACCOUNT")),
		Binary:        getEnvOrDefault("JOBCLAW_GOG_BIN", "gog"),
	})
	if err != nil {
		log.Fatalf("configure sheet writer: %v", err)
	}

	if err := writer.AppendHeader(ctx); err != nil {
		log.Fatalf("write sheet header: %v", err)
	}

	fmt.Println("Header written:")
	fmt.Printf("  %s\n", strings.Join(sheet.Header(), " | "))
}

// shortenReasoning keeps the score breakdown readable in a cell. The full string
// carries every component plus match ratios, which is more than a spreadsheet
// column can usefully show.
func shortenReasoning(reasoning string) string {
	const limit = 180

	reasoning = strings.TrimSpace(reasoning)

	if len(reasoning) <= limit {
		return reasoning
	}

	return reasoning[:limit] + "…"
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	if limit <= 1 {
		return value[:limit]
	}

	return value[:limit-1] + "…"
}
