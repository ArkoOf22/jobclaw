package metrics

import (
	"strings"
	"testing"
)

func TestSnapshotRenderIsDeterministicAndLabeled(t *testing.T) {
	snap := Snapshot{
		TotalJobs:  100,
		ScoredJobs: 90,
		JobsBySource: map[string]int{
			"linkedin": 60,
			"indeed":   30,
		},
		ShortlistBySource: map[string]int{
			"linkedin": 40,
			"indeed":   3,
		},
		JobsByRecommendation: map[string]int{
			"SHORTLIST": 43,
			"SKIP":      47,
		},
		ApplicationsByStatus: map[string]int{
			"APPLIED": 2,
		},
	}

	out := snap.Render()

	// Rendered twice, the output must be byte-identical: map iteration order
	// must not leak into the exposition, or Prometheus would see churn.
	if out != snap.Render() {
		t.Fatal("render output is not deterministic")
	}

	for _, want := range []string{
		"# TYPE jobclaw_jobs_total gauge",
		"jobclaw_jobs_total 100",
		"jobclaw_jobs_scored_total 90",
		`jobclaw_jobs_by_source{source="indeed"} 30`,
		`jobclaw_jobs_by_source{source="linkedin"} 60`,
		`jobclaw_shortlisted_by_source{source="linkedin"} 40`,
		`jobclaw_jobs_by_recommendation{recommendation="SHORTLIST"} 43`,
		`jobclaw_applications_by_status{status="APPLIED"} 2`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n---\n%s", want, out)
		}
	}
}

// An empty label value must not render as an empty string, which reads as
// missing data on a dashboard.
func TestSnapshotRenderSanitizesEmptySource(t *testing.T) {
	snap := Snapshot{JobsBySource: map[string]int{"": 5}}

	if !strings.Contains(snap.Render(), `jobclaw_jobs_by_source{source="unknown"} 5`) {
		t.Errorf("empty source not sanitized to 'unknown':\n%s", snap.Render())
	}
}
