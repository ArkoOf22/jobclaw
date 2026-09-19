// Package metrics exposes JobClaw pipeline health in the Prometheus text
// exposition format, using only the standard library.
//
// The Prometheus Go client would pull in a sizeable dependency tree, which cuts
// against this project's "few and deliberate dependencies" rule and matters on a
// 2GB host. The exposition format is a simple, stable text contract, so we emit
// it directly: Prometheus scrapes it identically whether it came from the
// official client or a hand-written renderer.
//
// The intended deployment is Option A: this endpoint runs on the host, and
// Prometheus + Grafana scrape it from off-box (e.g. over Tailscale), so the
// monitoring stack adds no memory pressure to the machine being monitored.
package metrics

import (
	"fmt"
	"sort"
	"strings"
)

// Snapshot is the set of pipeline facts rendered at scrape time. It is a plain
// data holder so the collector (which does the DB work) stays separate from the
// rendering, and so it is trivial to unit-test the exposition output.
type Snapshot struct {
	// JobsBySource counts stored jobs per discovery source (linkedin, indeed,
	// greenhouse, ...). This is the "how much is each source giving me" signal.
	JobsBySource map[string]int

	// ShortlistBySource counts jobs per source whose recommendation is
	// SHORTLIST or APPLY — the roles the source actually surfaced as worth a
	// decision, not just raw volume. Together with JobsBySource this yields the
	// per-source shortlist rate, the number that answers whether a source (or an
	// OpenClaw skill feeding one) is earning its keep.
	ShortlistBySource map[string]int

	// JobsByRecommendation counts all scored jobs per recommendation
	// (SHORTLIST, APPLY, SKIP).
	JobsByRecommendation map[string]int

	// ApplicationsByStatus counts applications per status (DRAFT, APPLIED, ...).
	ApplicationsByStatus map[string]int

	// TotalJobs and ScoredJobs let a dashboard show scoring coverage.
	TotalJobs  int
	ScoredJobs int
}

// Render returns the snapshot as Prometheus exposition text. Metric names are
// namespaced with jobclaw_ and follow Prometheus naming conventions.
func (s Snapshot) Render() string {
	var b strings.Builder

	writeGauge(&b,
		"jobclaw_jobs_total",
		"Total jobs stored in the database.",
		nil,
		float64(s.TotalJobs),
	)

	writeGauge(&b,
		"jobclaw_jobs_scored_total",
		"Jobs that have a score recorded.",
		nil,
		float64(s.ScoredJobs),
	)

	writeLabeledGauge(&b,
		"jobclaw_jobs_by_source",
		"Stored jobs partitioned by discovery source.",
		"source",
		s.JobsBySource,
	)

	writeLabeledGauge(&b,
		"jobclaw_shortlisted_by_source",
		"Jobs recommended SHORTLIST or APPLY, partitioned by discovery source.",
		"source",
		s.ShortlistBySource,
	)

	writeLabeledGauge(&b,
		"jobclaw_jobs_by_recommendation",
		"Scored jobs partitioned by recommendation.",
		"recommendation",
		s.JobsByRecommendation,
	)

	writeLabeledGauge(&b,
		"jobclaw_applications_by_status",
		"Applications partitioned by status.",
		"status",
		s.ApplicationsByStatus,
	)

	return b.String()
}

// writeGauge emits a single-value gauge with HELP and TYPE lines.
func writeGauge(
	b *strings.Builder,
	name, help string,
	_ map[string]string,
	value float64,
) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s gauge\n", name)
	fmt.Fprintf(b, "%s %s\n", name, formatValue(value))
}

// writeLabeledGauge emits one gauge metric with a series per map key. Keys are
// sorted so the output is deterministic, which keeps the exposition stable for
// tests and diffs.
func writeLabeledGauge(
	b *strings.Builder,
	name, help, label string,
	values map[string]int,
) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s gauge\n", name)

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(
			b,
			"%s{%s=%q} %d\n",
			name,
			label,
			sanitizeLabelValue(k),
			values[k],
		)
	}
}

// formatValue prints whole numbers without a trailing ".0" and keeps fractional
// values readable. Gauges here are all counts, but this keeps the door open.
func formatValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}

	return fmt.Sprintf("%g", v)
}

// sanitizeLabelValue guards against an empty source/status label, which would
// otherwise emit an empty string that reads as missing data on a dashboard.
func sanitizeLabelValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}

	return v
}
