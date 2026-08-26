package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"jobclaw/internal/application"
	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/sheet"
)

// Sheet status values. Distinct from job.Status because the sheet is a worklist
// the candidate reads, not a mirror of the state machine.
const (
	sheetStatusApplied     = "APPLIED"
	sheetStatusSkipped     = "SKIPPED"
	sheetStatusResumeReady = "RESUME READY"
)

// runMark records the outcome of a job the candidate acted on, updating both the
// database and the review sheet.
//
// This is the closing half of the workflow. Submission happens by hand, on the
// employer's own form, so nothing else can know it happened: the candidate has to
// say so. Without this, the sheet would grow indefinitely with rows that are all
// still marked NEW.
func runMark(
	jobID int64,
	outcome string,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)

	current, err := jobRepo.GetByID(ctx, jobID)
	if err != nil {
		log.Fatalf("load job: %v", err)
	}

	if current == nil {
		log.Fatalf("job %d not found", jobID)
	}

	var (
		sheetStatus string
		target      job.Status
	)

	switch outcome {
	case "applied":
		sheetStatus = sheetStatusApplied
		target = job.StatusApplied

	case "skipped":
		sheetStatus = sheetStatusSkipped
		target = job.StatusRejected

	default:
		log.Fatalf(
			"unknown outcome %q; expected applied or skipped",
			outcome,
		)
	}

	fmt.Println("JobClaw Mark")
	fmt.Println("────────────────────────────")
	fmt.Printf("Job:      %d\n", jobID)
	fmt.Printf("Company:  %s\n", current.Company)
	fmt.Printf("Title:    %s\n", current.Title)
	fmt.Println()

	if current.Status == target {
		fmt.Printf("Already %s in the database.\n", target)
	} else {
		// APPLIED is only reachable from APPROVED. A job applied to straight
		// off the sheet is usually still SCORED or SHORTLISTED, so bridge
		// through APPROVED rather than refusing: the candidate applying to it is
		// the approval.
		if target == job.StatusApplied &&
			current.Status != job.StatusApproved {
			if !job.CanTransition(current.Status, job.StatusApproved) {
				log.Fatalf(
					"cannot record an application for job %d from status %s",
					jobID,
					current.Status,
				)
			}

			if err := jobRepo.UpdateStatus(
				ctx,
				jobID,
				job.StatusApproved,
			); err != nil {
				log.Fatalf("approve job before marking applied: %v", err)
			}

			fmt.Printf("Status:   %s → APPROVED (implied by applying)\n",
				current.Status,
			)

			current.Status = job.StatusApproved
		}

		if !job.CanTransition(current.Status, target) {
			log.Fatalf(
				"invalid status transition: %s → %s",
				current.Status,
				target,
			)
		}

		if err := jobRepo.UpdateStatus(ctx, jobID, target); err != nil {
			log.Fatalf("update job status: %v", err)
		}

		fmt.Printf("Status:   %s → %s\n", current.Status, target)
	}

	if target == job.StatusApplied {
		syncApplicationApplied(ctx, jobID, db)
	}

	updateSheetStatus(ctx, jobID, sheet.RowUpdate{Status: sheetStatus})
}

// syncApplicationApplied advances the application row that belongs to a job the
// candidate has now applied to.
//
// A job can be applied to straight off the sheet, in which case there is no
// application row and nothing to do. But when one exists it was created by the
// resume pipeline and still reads READY_TO_APPLY, which contradicts the job it
// points at. Anything reading application status would report the job as still
// pending, so the two are kept in step.
//
// Best-effort, like the sheet update: the job status change has already
// committed, and losing it to a follow-up failure would be worse than a stale
// application row.
func syncApplicationApplied(
	ctx context.Context,
	jobID int64,
	db *database.DB,
) {
	appRepo := application.NewSQLiteRepository(db)

	existing, err := appRepo.GetByJobID(ctx, jobID)
	if err != nil {
		fmt.Printf("Applic.:  not updated (%v)\n", err)

		return
	}

	if existing == nil {
		return
	}

	if existing.Status == application.StatusApplied {
		fmt.Printf("Applic.:  %d already APPLIED\n", existing.ID)

		return
	}

	if err := appRepo.UpdateStatus(
		ctx,
		existing.ID,
		application.StatusApplied,
	); err != nil {
		fmt.Printf("Applic.:  not updated (%v)\n", err)

		return
	}

	fmt.Printf(
		"Applic.:  %d %s → APPLIED\n",
		existing.ID,
		existing.Status,
	)
}

// updateSheetStatus mirrors an outcome onto the sheet.
//
// Best-effort by design: the database is the source of truth, so a Google API
// failure must not lose the state change that already succeeded. It reports what
// happened rather than failing the command.
func updateSheetStatus(
	ctx context.Context,
	jobID int64,
	update sheet.RowUpdate,
) {
	spreadsheetID := strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ID"))

	if spreadsheetID == "" {
		fmt.Println("Sheet:    not configured, skipped")

		return
	}

	writer, err := sheet.NewGogWriter(sheet.Config{
		SpreadsheetID: spreadsheetID,
		SheetName:     getEnvOrDefault("JOBCLAW_SHEET_TAB", "Jobs"),
		Account:       strings.TrimSpace(os.Getenv("JOBCLAW_SHEET_ACCOUNT")),
		Binary:        getEnvOrDefault("JOBCLAW_GOG_BIN", "gog"),
	})
	if err != nil {
		fmt.Printf("Sheet:    not updated (%v)\n", err)

		return
	}

	found, err := writer.UpdateJobRow(ctx, jobID, update)
	if err != nil {
		fmt.Printf("Sheet:    not updated (%v)\n", err)
		fmt.Println("          The database change stands; re-run to retry.")

		return
	}

	if !found {
		fmt.Println("Sheet:    no row for this job yet")

		return
	}

	if update.Status != "" {
		fmt.Printf("Sheet:    status set to %s\n", update.Status)
	}

	if update.ResumeLink != "" {
		fmt.Printf("Sheet:    resume link recorded\n")
	}
}
