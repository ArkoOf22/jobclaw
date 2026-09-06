package discovery

import (
	"context"
	"errors"
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

// Partial reports a source that returned usable jobs and an error together.
//
// A composite source such as Greenhouse reads several boards, and one board
// failing says nothing about the rest. Collapsing that into a plain failure hid
// the fact that results were still available.
func (r Result) Partial() bool {
	return r.Failed && r.Stored > 0
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

			// A source may return jobs *and* an error, when it aggregates
			// several upstreams and only some of them failed. Storing whatever
			// arrived before reporting the error keeps a partial failure from
			// throwing away work that succeeded, while still surfacing the
			// failure. Returning early here used to discard every job collected
			// from the healthy upstreams.
			if err != nil {
				result.Failed = true
				result.Error = err
			}

			result.Fetched = len(jobs)

			for _, candidate := range jobs {
				if storeErr := s.repository.Upsert(ctx, candidate); storeErr != nil {
					result.Failed = true
					result.Error = errors.Join(
						result.Error,
						fmt.Errorf("store job: %w", storeErr),
					)

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
