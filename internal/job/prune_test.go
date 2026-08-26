package job

import (
	"context"
	"testing"
	"time"

	"jobclaw/internal/database"
)

func setupPruneTest(t *testing.T) (*database.DB, *SQLiteRepository) {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatal(err)
	}

	return db, NewSQLiteRepository(db)
}

func seedPruneJob(
	t *testing.T,
	repo *SQLiteRepository,
	externalID string,
	status Status,
) int64 {
	t.Helper()

	ctx := context.Background()

	if err := NewSQLiteSourceRepository(repo.db).EnsureSource(
		ctx,
		"greenhouse",
	); err != nil {
		t.Fatal(err)
	}

	if err := repo.Upsert(ctx, Job{
		Source:     "greenhouse",
		ExternalID: externalID,
		Company:    "Acme",
		Title:      "Backend Engineer",
		URL:        "https://example.com/" + externalID,
	}); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.GetBySourceExternalID(
		ctx,
		"greenhouse",
		externalID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if stored == nil {
		t.Fatalf("job %s was not stored", externalID)
	}

	if status != StatusDiscovered {
		if err := repo.UpdateStatus(ctx, stored.ID, status); err != nil {
			t.Fatal(err)
		}
	}

	return stored.ID
}

// A shortlisted job is an open decision. Pruning it would destroy the queue
// rather than tidy it, so it must never be eligible.
func TestFindPrunableExcludesShortlisted(t *testing.T) {
	db, repo := setupPruneTest(t)
	defer db.Close()

	ctx := context.Background()

	scoredID := seedPruneJob(t, repo, "scored-1", StatusScored)
	shortlistedID := seedPruneJob(t, repo, "short-1", StatusShortlisted)

	candidates, err := repo.FindPrunable(ctx, PruneCriteria{
		Before: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := map[int64]bool{}

	for _, candidate := range candidates {
		found[candidate.ID] = true
	}

	if !found[scoredID] {
		t.Fatal("a SCORED job should be prunable")
	}

	if found[shortlistedID] {
		t.Fatal("a SHORTLISTED job must never be prunable")
	}
}

// Statuses that may carry an application or a submission must be refused
// outright rather than silently returning nothing.
func TestFindPrunableRejectsProtectedStatuses(t *testing.T) {
	db, repo := setupPruneTest(t)
	defer db.Close()

	for _, status := range []Status{
		StatusShortlisted,
		StatusApproved,
		StatusApplied,
		StatusInterview,
		StatusOffer,
	} {
		t.Run(string(status), func(t *testing.T) {
			_, err := repo.FindPrunable(
				context.Background(),
				PruneCriteria{
					Before:   time.Now().UTC(),
					Statuses: []Status{status},
				},
			)

			if err == nil {
				t.Fatalf("pruning %s should be refused", status)
			}
		})
	}
}

func TestFindPrunableRequiresCutoff(t *testing.T) {
	db, repo := setupPruneTest(t)
	defer db.Close()

	if _, err := repo.FindPrunable(
		context.Background(),
		PruneCriteria{},
	); err == nil {
		t.Fatal("a missing cutoff should be refused")
	}
}

// The cutoff must actually exclude recent jobs.
func TestFindPrunableHonoursCutoff(t *testing.T) {
	db, repo := setupPruneTest(t)
	defer db.Close()

	ctx := context.Background()

	seedPruneJob(t, repo, "recent-1", StatusScored)

	candidates, err := repo.FindPrunable(ctx, PruneCriteria{
		// Everything was just created, so a cutoff in the past matches nothing.
		Before: time.Now().UTC().Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(candidates) != 0 {
		t.Fatalf(
			"candidates = %d, want 0 for a past cutoff",
			len(candidates),
		)
	}
}

// DeleteJobs re-checks each row inside the transaction, so a job that became
// protected between preview and delete survives.
func TestDeleteJobsSkipsProtectedRows(t *testing.T) {
	db, repo := setupPruneTest(t)
	defer db.Close()

	ctx := context.Background()

	prunableID := seedPruneJob(t, repo, "prunable-1", StatusScored)
	protectedID := seedPruneJob(t, repo, "protected-1", StatusScored)

	// Simulate the row advancing after the preview was taken.
	if err := repo.UpdateStatus(
		ctx,
		protectedID,
		StatusApproved,
	); err != nil {
		t.Fatal(err)
	}

	deleted, err := repo.DeleteJobs(ctx, []int64{prunableID, protectedID})
	if err != nil {
		t.Fatal(err)
	}

	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	gone, err := repo.GetByID(ctx, prunableID)
	if err != nil {
		t.Fatal(err)
	}

	if gone != nil {
		t.Fatal("the prunable job should have been deleted")
	}

	survivor, err := repo.GetByID(ctx, protectedID)
	if err != nil {
		t.Fatal(err)
	}

	if survivor == nil {
		t.Fatal("the newly approved job must survive")
	}
}

func TestDeleteJobsWithNoIDsIsANoOp(t *testing.T) {
	db, repo := setupPruneTest(t)
	defer db.Close()

	deleted, err := repo.DeleteJobs(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}
