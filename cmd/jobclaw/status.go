package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
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
	//
	// This is the most actionable slice, not the whole backlog. Ordered by
	// recommendation (APPLY, then SHORTLIST, then the rest) and by score within
	// each group, capped at defaultAwaitingApprovalLimit and overridable with
	// --limit. AwaitingApprovalTotal carries the real count.
	//
	// It used to return every row. With 2,150 pending jobs that made this
	// document 597KB, or roughly 149,000 tokens, and an agent that runs
	// `status --json` swallows all of it on every call. That single field was
	// the largest cost in the whole system and produced observed requests of
	// over 500,000 prompt tokens. A human cannot act on 2,150 rows anyway, so
	// the full list was expensive and useless at the same time.
	AwaitingApproval []JobSummary `json:"awaiting_approval"`

	// AwaitingApprovalTotal is how many jobs actually await a decision,
	// regardless of how many are listed above. Always the true count, so a
	// caller can tell a capped list from a short one.
	AwaitingApprovalTotal int `json:"awaiting_approval_total"`

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

// recommendationRank orders jobs by how worth deciding on they are, lowest
// first. A recommendation is advice rather than a wall — SCORED -> APPROVED is a
// legal transition, so a SKIP can still be approved by hand — but it belongs
// below everything actionable.
func recommendationRank(summary JobSummary) int {
	switch summary.Recommendation {
	case string(scoring.RecommendationApply):
		return 0
	case string(scoring.RecommendationShortlist):
		return 1
	case "":
		// Not scored yet, so there is nothing to act on.
		return 3
	default:
		return 2
	}
}

// defaultAwaitingApprovalLimit caps how many pending jobs `status` lists.
//
// Chosen to be more than a person will work through in one sitting and small
// enough that an agent reading this output pays almost nothing for it. Pass
// --limit 0 for the full backlog when something genuinely needs every row.
const defaultAwaitingApprovalLimit = 25

func runStatus(
	databasePath string,
	asJSON bool,
	limit int,
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

	// status is documented as the place to get real totals, in contrast to
	// `jobs list` showing only the first 20. A hardcoded 1000 quietly broke that
	// promise once the table passed 1000 rows: it reported "Jobs (1000)" against
	// 1347 stored, and the missing rows were the oldest, which are exactly the
	// ones whose scores are most likely to be stale.
	total, err := jobRepo.Count(ctx)
	if err != nil {
		log.Fatalf("count jobs: %v", err)
	}

	jobs, err := jobRepo.List(ctx, total)
	if err != nil {
		log.Fatalf("list jobs: %v", err)
	}

	scores, err := scoreRepo.List(ctx, total)
	if err != nil {
		log.Fatalf("list scores: %v", err)
	}

	applications, err := applicationRepo.List(ctx, 1000)
	if err != nil {
		log.Fatalf("list applications: %v", err)
	}

	scoreByJob := make(map[int64]scoring.StoredScore, len(scores))

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

	// The true count, recorded before the list is trimmed.
	report.AwaitingApprovalTotal = len(report.AwaitingApproval)

	// Actionable first, then highest score within each group.
	//
	// Score alone is not the right order. A vetoed job keeps whatever score its
	// stack and location earned, so sorting on score alone puts SKIPs at the top
	// whenever a senior role happens to match the tech well — which is exactly
	// the case the veto exists to remove. The list then still reads as "senior
	// roles first", just with a SKIP label, and the reader is back to filtering
	// by hand.
	//
	// Unscored entries sort last within their group: they cannot be judged yet,
	// so they are the least useful thing to spend the cap on.
	sort.SliceStable(report.AwaitingApproval, func(i, j int) bool {
		left, right := report.AwaitingApproval[i], report.AwaitingApproval[j]

		if lr, rr := recommendationRank(left), recommendationRank(right); lr != rr {
			return lr < rr
		}

		switch {
		case left.Score == nil && right.Score == nil:
			return false
		case left.Score == nil:
			return false
		case right.Score == nil:
			return true
		default:
			return *left.Score > *right.Score
		}
	})

	if limit > 0 && len(report.AwaitingApproval) > limit {
		report.AwaitingApproval = report.AwaitingApproval[:limit]
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
		job.StatusScored,
		job.StatusShortlisted,
		job.StatusApproved,
		job.StatusApplied,
		job.StatusInterview,
		job.StatusOffer,
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

	// Say when the list is a slice rather than the whole thing, otherwise the
	// header count and the number of rows disagree for no visible reason.
	if report.AwaitingApprovalTotal > len(report.AwaitingApproval) {
		fmt.Printf(
			"Awaiting your approval (%d, showing top %d: actionable first)\n",
			report.AwaitingApprovalTotal,
			len(report.AwaitingApproval),
		)
	} else {
		fmt.Printf(
			"Awaiting your approval (%d)\n",
			report.AwaitingApprovalTotal,
		)
	}

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

// runPrune retires stale jobs. Preview is the default; deleting requires
// --confirm, the same gate submission uses, because deletion is irreversible.
func runPrune(
	olderThanDays int,
	confirmed bool,
	db *database.DB,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	jobRepo := job.NewSQLiteRepository(db)

	cutoff := time.Now().UTC().AddDate(0, 0, -olderThanDays)

	candidates, err := jobRepo.FindPrunable(
		ctx,
		job.PruneCriteria{Before: cutoff},
	)
	if err != nil {
		log.Fatalf("find prunable jobs: %v", err)
	}

	fmt.Println("JobClaw Prune")
	fmt.Println("────────────────────────────")
	fmt.Printf(
		"Cutoff:     jobs discovered before %s (%d days ago)\n",
		cutoff.Format("2006-01-02"),
		olderThanDays,
	)
	fmt.Printf("Eligible:   %d\n", len(candidates))
	fmt.Println()
	fmt.Println("Never eligible: SHORTLISTED jobs (an open decision), jobs with")
	fmt.Println("an application, and anything at APPROVED, APPLIED, INTERVIEW,")
	fmt.Println("or OFFER. Reject a shortlisted job first if you want it gone.")
	fmt.Println()

	if len(candidates) == 0 {
		fmt.Println("Nothing to prune.")

		return
	}

	byStatus := map[job.Status]int{}

	for _, candidate := range candidates {
		byStatus[candidate.Status]++
	}

	fmt.Println("By status")

	for status, count := range byStatus {
		fmt.Printf("  %-14s %d\n", status, count)
	}

	fmt.Println()

	const sampleSize = 10

	fmt.Printf("Sample (first %d)\n", sampleSize)

	for i, candidate := range candidates {
		if i >= sampleSize {
			break
		}

		fmt.Printf(
			"  [%d] %s — %s (%s, %s)\n",
			candidate.ID,
			candidate.Company,
			candidate.Title,
			candidate.Status,
			candidate.DiscoveredAt.Format("2006-01-02"),
		)
	}

	fmt.Println()

	if !confirmed {
		fmt.Println("────────────────────────────")
		fmt.Println("Mode: DRY RUN — nothing was deleted")
		fmt.Println()
		fmt.Printf(
			"To delete these %d jobs and their scores:\n",
			len(candidates),
		)
		fmt.Printf(
			"  jobclaw prune --older-than %d --confirm\n",
			olderThanDays,
		)
		fmt.Println()
		fmt.Println("Back up data/jobclaw.db first. This cannot be undone.")

		return
	}

	ids := make([]int64, 0, len(candidates))

	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
	}

	deleted, err := jobRepo.DeleteJobs(ctx, ids)
	if err != nil {
		log.Fatalf("prune jobs: %v", err)
	}

	fmt.Println("────────────────────────────")
	fmt.Printf("Deleted %d job(s) and their scores.\n", deleted)

	if deleted != int64(len(candidates)) {
		// The guard inside the transaction skipped rows that changed after the
		// preview was taken.
		fmt.Printf(
			"%d candidate(s) were skipped because they changed since the preview.\n",
			int64(len(candidates))-deleted,
		)
	}
}
