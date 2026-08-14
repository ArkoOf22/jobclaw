package discovery

import (
	"context"
	"fmt"
	"time"

	"jobclaw/internal/job"
)

type Service struct {
	repository job.Repository
	sources    []Source
}

func NewService(repository job.Repository, sources ...Source) *Service {
	return &Service{
		repository: repository,
		sources:    sources,
	}
}

type Result struct {
	Source   string
	Fetched  int
	Stored   int
	Failed   bool
	Error    error
	Duration time.Duration
}

func (s *Service) Discover(
	ctx context.Context,
	request Request,
) []Result {
	results := make([]Result, 0, len(s.sources))

	for _, source := range s.sources {
		start := time.Now()

		jobs, err := source.Discover(ctx, request)

		result := Result{
			Source:   source.Name(),
			Duration: time.Since(start),
		}

		if err != nil {
			result.Failed = true
			result.Error = err
			results = append(results, result)
			continue
		}

		result.Fetched = len(jobs)

		for _, j := range jobs {
			if err := s.repository.Upsert(ctx, j); err != nil {
				result.Failed = true
				result.Error = fmt.Errorf("store job: %w", err)
				break
			}

			result.Stored++
		}

		result.Duration = time.Since(start)
		results = append(results, result)
	}

	return results
}
