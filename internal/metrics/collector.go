package metrics

import (
	"context"
	"fmt"

	"jobclaw/internal/application"
	"jobclaw/internal/job"
	"jobclaw/internal/scoring"
)

// Collector reads the current pipeline state from the repositories into a
// Snapshot. It holds no state of its own and does a fresh read per Collect, so
// every scrape reflects the live database — the same discipline the agent skill
// enforces for job data.
type Collector struct {
	jobs  *job.SQLiteRepository
	score *scoring.Repository
	apps  *application.SQLiteRepository
}

func NewCollector(
	jobs *job.SQLiteRepository,
	score *scoring.Repository,
	apps *application.SQLiteRepository,
) *Collector {
	return &Collector{jobs: jobs, score: score, apps: apps}
}

// Collect builds a Snapshot from the current database contents.
//
// It reads the whole jobs table rather than a window: the per-source shortlist
// rate is only meaningful over everything, and a capped read would silently
// undercount older sources. The table is small enough (low thousands) that a
// full read per scrape is fine at a sane scrape interval.
func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	total, err := c.jobs.Count(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("count jobs: %w", err)
	}

	jobs, err := c.jobs.List(ctx, total)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list jobs: %w", err)
	}

	scores, err := c.score.List(ctx, total)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list scores: %w", err)
	}

	apps, err := c.apps.List(ctx, 1000)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list applications: %w", err)
	}

	scoreByJob := make(map[int64]scoring.StoredScore, len(scores))
	for _, s := range scores {
		scoreByJob[s.JobID] = s
	}

	snap := Snapshot{
		JobsBySource:         map[string]int{},
		ShortlistBySource:    map[string]int{},
		JobsByRecommendation: map[string]int{},
		ApplicationsByStatus: map[string]int{},
		TotalJobs:            len(jobs),
	}

	for _, j := range jobs {
		snap.JobsBySource[j.Source]++

		score, ok := scoreByJob[j.ID]
		if !ok {
			continue
		}

		snap.ScoredJobs++

		rec := string(score.Recommendation)
		snap.JobsByRecommendation[rec]++

		// A source's value is the roles it surfaces as worth a decision, so
		// APPLY counts alongside SHORTLIST here.
		if score.Recommendation == scoring.RecommendationShortlist ||
			score.Recommendation == scoring.RecommendationApply {
			snap.ShortlistBySource[j.Source]++
		}
	}

	for _, a := range apps {
		snap.ApplicationsByStatus[string(a.Status)]++
	}

	return snap, nil
}
