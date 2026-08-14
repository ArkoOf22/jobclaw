package discovery

import (
	"context"
	"testing"

	"jobclaw/internal/job"
)

type fakeSource struct {
	name string
	jobs []job.Job
	err  error
}

func (f *fakeSource) Name() string {
	return f.name
}

func (f *fakeSource) Discover(
	ctx context.Context,
	request Request,
) ([]job.Job, error) {
	return f.jobs, f.err
}

type fakeRepository struct {
	jobs []job.Job
}

func (r *fakeRepository) Upsert(
	ctx context.Context,
	j job.Job,
) error {
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
	return r.jobs, nil
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

	if len(repository.jobs) != 2 {
		t.Fatalf("expected 2 repository jobs, got %d", len(repository.jobs))
	}
}
