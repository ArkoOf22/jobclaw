package discovery

import (
	"context"
	"fmt"
	"sync"
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
	results := make([]Result, len(s.sources))

	var wg sync.WaitGroup

	for i, source := range s.sources {
		wg.Add(1)

		go func(index int, source Source) {
			defer wg.Done()

			start := time.Now()

			jobs, err := source.Discover(ctx, request)

			result := Result{
				Source:   source.Name(),
				Duration: time.Since(start),
			}

			if err != nil {
				result.Failed = true
				result.Error = err
				result.Duration = time.Since(start)
				results[index] = result
				return
			}

			result.Fetched = len(jobs)

			for _, candidate := range jobs {
				if err := s.repository.Upsert(ctx, candidate); err != nil {
					result.Failed = true
					result.Error = fmt.Errorf("store job: %w", err)
					break
				}

				result.Stored++
			}

			result.Duration = time.Since(start)
			results[index] = result
		}(i, source)
	}

	wg.Wait()

	return results
}
