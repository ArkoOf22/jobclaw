package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/sheet"
)

// rebuildBatchSize matches the page size sheet sync uses. The rebuild loops
// until the queue drains, so this only bounds how much is held at once.
const rebuildBatchSize = 200

// rebuildMaxBatches stops a runaway loop. At 200 a batch this is 4,000 rows,
// far beyond any plausible backlog, so hitting it means something is wrong —
// most likely rows being written without being marked synced.
const rebuildMaxBatches = 20

// runSheetRebuild empties the review sheet and writes it again from current
// scores.
//
// Needed because the sheet is append-only and its rows never revise themselves.
// A row's verdict can change after it was written — rescoring under tighter rules
// turns a SHORTLIST into a SKIP — and `sheet sync` skips anything already marked
// synced. The result is a sheet that keeps advertising jobs the scorer has since
// rejected, which is worse than an empty one: the reader trusts it and spends
// their time opening dead links.
//
// Destructive, so it is gated behind --confirm like submit and prune, and it
// snapshots the existing contents to data/backups/ first. The snapshot matters
// beyond paranoia: the sheet holds things JobClaw does not model, including
// anything typed into a cell by hand.
//
// One limitation is deliberate and reported rather than worked around. Only
// SHORTLIST and APPLY rows are re-added, so a job that was applied to and has
// since been rescored to SKIP will not come back. The database still knows about
// it; the sheet will not. Those jobs are listed at the end so the gap is visible
// instead of silent.
func runSheetRebuild(
	confirm bool,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Minute,
	)
	defer cancel()

	spreadsheetID := strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ID"))

	if spreadsheetID == "" {
		log.Fatal(
			"JOBCLAW_SHEET_ID is not set; create a sheet and put its ID in .env",
		)
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

	jobRepo := job.NewSQLiteRepository(db)

	eligible, err := jobRepo.CountSheetEligible(ctx)
	if err != nil {
		log.Fatalf("count eligible jobs: %v", err)
	}

	stranded, err := jobRepo.ListSheetStranded(ctx)
	if err != nil {
		log.Fatalf("list stranded jobs: %v", err)
	}

	fmt.Println("JobClaw Sheet Rebuild")
	fmt.Println("────────────────────────────")
	fmt.Printf("Spreadsheet:   %s\n", spreadsheetID)
	fmt.Printf("Rows to write: %d (SHORTLIST or APPLY, not rejected)\n", eligible)

	if len(stranded) > 0 {
		fmt.Printf(
			"Will NOT return: %d job(s) past the approval gate that now score SKIP\n",
			len(stranded),
		)
	}

	if !confirm {
		fmt.Println()
		fmt.Println("Preview only. Nothing was cleared and nothing was written.")
		fmt.Println("The sheet is emptied and rewritten only with --confirm.")

		if len(stranded) > 0 {
			printStranded(stranded)
		}

		return
	}

	fmt.Println()
	fmt.Println("Snapshotting current sheet...")

	snapshot, err := writer.Snapshot(ctx)
	if err != nil {
		log.Fatalf("snapshot sheet: %v", err)
	}

	backupPath, err := writeSheetBackup(snapshot)
	if err != nil {
		log.Fatalf("write sheet backup: %v", err)
	}

	fmt.Printf("  saved %d bytes to %s\n", len(snapshot), backupPath)

	fmt.Println("Clearing data rows (header kept)...")

	if err := writer.Clear(ctx); err != nil {
		log.Fatalf("clear sheet: %v", err)
	}

	// Only after the sheet is actually empty. Resetting first and then failing to
	// clear would duplicate every row on the next sync.
	reset, err := jobRepo.ResetSheetSync(ctx)
	if err != nil {
		log.Fatalf("reset sheet sync state: %v", err)
	}

	fmt.Printf("  reset sync state on %d row(s)\n", reset)
	fmt.Println("Writing rows...")

	written := 0

	for batch := 1; batch <= rebuildMaxBatches; batch++ {
		candidates, err := jobRepo.ListUnsyncedForSheet(ctx, rebuildBatchSize)
		if err != nil {
			log.Fatalf("list jobs for sheet: %v", err)
		}

		if len(candidates) == 0 {
			break
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
				ResumeLink:     "",
				Status:         sheetStatusFor(candidate.Job.Status),
				Reasoning:      shortenReasoning(candidate.Reasoning),
				DiscoveredAt:   candidate.Job.DiscoveredAt,
			})

			ids = append(ids, candidate.Job.ID)
		}

		if err := writer.Append(ctx, rows); err != nil {
			log.Fatalf("append to sheet: %v", err)
		}

		if err := jobRepo.MarkSheetSynced(ctx, ids); err != nil {
			log.Fatalf(
				"rows were written but could not be marked synced, "+
					"so a re-run would duplicate them: %v",
				err,
			)
		}

		written += len(rows)

		fmt.Printf("  batch %d: %d row(s), %d total\n", batch, len(rows), written)
	}

	fmt.Println()
	fmt.Printf("Rebuilt: %d row(s) written.\n", written)

	if written != eligible {
		fmt.Printf(
			"Note: expected %d, wrote %d. New jobs can be scored mid-run, "+
				"so a small difference is normal.\n",
			eligible,
			written,
		)
	}

	if len(stranded) > 0 {
		printStranded(stranded)
	}
}

// sheetStatusFor seeds the sheet's Status cell from the job's real status rather
// than always writing NEW.
//
// A rebuild is not a fresh discovery. Writing NEW over a job already approved or
// applied to would lose exactly the information the candidate uses the sheet to
// track.
func sheetStatusFor(status job.Status) string {
	switch status {
	case job.StatusApproved:
		return "APPROVED"
	case job.StatusApplied:
		return "APPLIED"
	default:
		return "NEW"
	}
}

func printStranded(stranded []job.SheetCandidate) {
	fmt.Println()
	fmt.Println("These will not appear on the rebuilt sheet: they are past the")
	fmt.Println("approval gate but have no score, so there is no verdict to write.")

	for _, candidate := range stranded {
		fmt.Printf(
			"  [%d] %-10s %5.1f %-22s %s\n",
			candidate.Job.ID,
			candidate.Job.Status,
			candidate.OverallScore,
			truncate(candidate.Job.Company, 22),
			truncate(candidate.Job.Title, 40),
		)
	}

	fmt.Println()
	fmt.Println("They are still in the database; `jobclaw job <id>` shows them.")
}

// writeSheetBackup stores the pre-rebuild contents next to the database backups,
// so the two are found together when something needs reconstructing.
func writeSheetBackup(snapshot []byte) (string, error) {
	directory := filepath.Join("data", "backups")

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	path := filepath.Join(
		directory,
		fmt.Sprintf("sheet-%s.json", time.Now().UTC().Format("20060102-150405")),
	)

	if err := os.WriteFile(path, snapshot, 0o644); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	return path, nil
}
