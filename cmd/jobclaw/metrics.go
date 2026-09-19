package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"jobclaw/internal/application"
	"jobclaw/internal/database"
	"jobclaw/internal/job"
	"jobclaw/internal/metrics"
	"jobclaw/internal/scoring"
)

// runMetrics starts a small HTTP server exposing /metrics in Prometheus text
// format, so an off-box Prometheus + Grafana can scrape JobClaw pipeline health
// without adding memory pressure to this host (Option A).
//
// It binds to a configurable address (default 127.0.0.1:9090). Keep it on
// loopback or a Tailscale address; there is nothing secret in the metrics, but
// there is no auth either, so it should not face the public internet.
func runMetrics(addr string, db *database.DB) {
	jobRepo := job.NewSQLiteRepository(db)
	scoreRepo := scoring.NewRepository(db)
	appRepo := application.NewSQLiteRepository(db)

	collector := metrics.NewCollector(jobRepo, scoreRepo, appRepo)

	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// A scrape must not hang the server if the DB is slow; bound it well
		// under a typical 15s Prometheus scrape timeout.
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		snap, err := collector.Collect(ctx)
		if err != nil {
			log.Printf("metrics collect failed: %v", err)
			http.Error(
				w,
				"# metrics collection failed\n",
				http.StatusInternalServerError,
			)

			return
		}

		w.Header().Set(
			"Content-Type",
			"text/plain; version=0.0.4; charset=utf-8",
		)

		fmt.Fprint(w, snap.Render())
	})

	// A trivial liveness endpoint, handy for confirming the server is up
	// without triggering a full DB read.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Println("JobClaw Metrics")
	fmt.Println("────────────────────────────")
	fmt.Printf("Serving /metrics on http://%s/metrics\n", addr)
	fmt.Println("Scrape this from an off-box Prometheus (e.g. over Tailscale).")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("metrics server: %v", err)
	}
}
