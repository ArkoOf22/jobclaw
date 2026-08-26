package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"jobclaw/internal/job"
)

type fakeSource struct {
	name  string
	jobs  []job.Job
	err   error
	delay time.Duration
}

func (f *fakeSource) Name() string {
	return f.name
}

func (f *fakeSource) Discover(
	ctx context.Context,
	request Request,
) ([]job.Job, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return f.jobs, f.err
}

type fakeRepository struct {
	mu   sync.Mutex
	jobs []job.Job
}

func (r *fakeRepository) Upsert(
	ctx context.Context,
	j job.Job,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.jobs = append(r.jobs, j)
	return nil
}

func (r *fakeRepository) GetBySourceExternalID(
	ctx context.Context,
	source string,
	externalID string,
) (*job.Job, error) {
	return nil, nil
}

func (r *fakeRepository) List(
	ctx context.Context,
	limit int,
) ([]job.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]job.Job(nil), r.jobs...), nil
}

func (r *fakeRepository) GetByID(
	ctx context.Context,
	id int64,
) (*job.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.jobs {
		if r.jobs[i].ID == id {
			found := r.jobs[i]
			return &found, nil
		}
	}

	return nil, nil
}

func (r *fakeRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status job.Status,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].Status = status
			return nil
		}
	}

	return nil
}

func TestServiceDiscoversAndStoresJobs(t *testing.T) {
	repository := &fakeRepository{}

	source := &fakeSource{
		name: "jobspy",
		jobs: []job.Job{
			{
				Source:     "jobspy",
				ExternalID: "123",
				Company:    "Example Product",
				Title:      "Backend Engineer",
				URL:        "https://example.com/jobs/123",
			},
			{
				Source:     "jobspy",
				ExternalID: "456",
				Company:    "Example Product",
				Title:      "Software Engineer",
				URL:        "https://example.com/jobs/456",
			},
		},
	}

	service := NewService(repository, source)

	results := service.Discover(
		context.Background(),
		Request{
			Keywords:  []string{"Backend Engineer"},
			Locations: []string{"Bangalore"},
			HoursOld:  72,
			Limit:     100,
		},
	)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Fetched != 2 {
		t.Fatalf("expected 2 fetched jobs, got %d", results[0].Fetched)
	}

	if results[0].Stored != 2 {
		t.Fatalf("expected 2 stored jobs, got %d", results[0].Stored)
	}

	repository.mu.Lock()
	defer repository.mu.Unlock()

	if len(repository.jobs) != 2 {
		t.Fatalf("expected 2 repository jobs, got %d", len(repository.jobs))
	}
}

func TestServiceContinuesWhenOneSourceFails(t *testing.T) {
	repository := &fakeRepository{}

	failingSource := &fakeSource{
		name: "jobspy",
		err:  errors.New("jobspy unavailable"),
	}

	successfulSource := &fakeSource{
		name: "greenhouse",
		jobs: []job.Job{
			{
				Source:     "greenhouse",
				ExternalID: "123",
				Company:    "Example Company",
				Title:      "Backend Engineer",
				URL:        "https://example.com/jobs/123",
			},
		},
	}

	service := NewService(
		repository,
		failingSource,
		successfulSource,
	)

	results := service.Discover(
		context.Background(),
		Request{},
	)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Source != "jobspy" {
		t.Fatalf(
			"expected first result to be jobspy, got %s",
			results[0].Source,
		)
	}

	if !results[0].Failed {
		t.Fatal("expected jobspy result to fail")
	}

	if results[1].Source != "greenhouse" {
		t.Fatalf(
			"expected second result to be greenhouse, got %s",
			results[1].Source,
		)
	}

	if results[1].Failed {
		t.Fatalf(
			"expected greenhouse result to succeed: %v",
			results[1].Error,
		)
	}

	if results[1].Stored != 1 {
		t.Fatalf(
			"expected 1 stored greenhouse job, got %d",
			results[1].Stored,
		)
	}
}

func TestServiceRunsSourcesConcurrently(t *testing.T) {
	repository := &fakeRepository{}

	first := &fakeSource{
		name:  "first",
		delay: 200 * time.Millisecond,
	}

	second := &fakeSource{
		name:  "second",
		delay: 200 * time.Millisecond,
	}

	service := NewService(repository, first, second)

	start := time.Now()

	results := service.Discover(
		context.Background(),
		Request{},
	)

	duration := time.Since(start)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Sequential execution would take roughly 400ms.
	// Allow enough margin for slow CI while still proving overlap.
	if duration >= 350*time.Millisecond {
		t.Fatalf(
			"expected concurrent execution, took %s",
			duration,
		)
	}

	// Result order should follow source order, not completion order.
	if results[0].Source != "first" {
		t.Fatalf(
			"expected first result to be first, got %s",
			results[0].Source,
		)
	}

	if results[1].Source != "second" {
		t.Fatalf(
			"expected second result to be second, got %s",
			results[1].Source,
		)
	}
}
