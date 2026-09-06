package greenhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"jobclaw/internal/discovery"
	"jobclaw/internal/job"
)

// MultiBoardClient discovers jobs from multiple Greenhouse boards.
type MultiBoardClient struct {
	clients []*Client
}

// NewMultiBoardClient creates a Greenhouse discovery source for all
// non-empty board tokens provided.
func NewMultiBoardClient(boardTokens []string) *MultiBoardClient {
	clients := make([]*Client, 0, len(boardTokens))

	for _, boardToken := range boardTokens {
		boardToken = strings.TrimSpace(boardToken)
		if boardToken == "" {
			continue
		}

		clients = append(clients, NewClient(boardToken))
	}

	return &MultiBoardClient{
		clients: clients,
	}
}

func (c *MultiBoardClient) Name() string {
	return "greenhouse"
}

// Discover fetches from every configured board, isolating per-board failures.
//
// A board is an independent employer, so one bad token must not cost the others.
// This previously returned on the first error and discarded the jobs already
// collected, which meant a single retired board token silently reduced the whole
// Greenhouse source to zero results. The boards are behind one discovery.Source,
// so the service's own source isolation cannot help here: the isolation has to
// happen at this level.
//
// Partial results are returned alongside a joined error, so the caller can both
// store what succeeded and report what did not. The board token is included in
// each error, since "greenhouse board returned HTTP 404" is not actionable
// without knowing which of a dozen boards produced it.
func (c *MultiBoardClient) Discover(
	ctx context.Context,
	request discovery.Request,
) ([]job.Job, error) {
	if len(c.clients) == 0 {
		return nil, fmt.Errorf("no greenhouse board tokens configured")
	}

	allJobs := make([]job.Job, 0)

	var boardErrs []error

	for _, client := range c.clients {
		jobs, err := client.Discover(ctx, request)
		if err != nil {
			boardErrs = append(boardErrs, fmt.Errorf(
				"board %q: %w",
				client.BoardToken(),
				err,
			))

			// Abandon the remaining boards only when the run itself is over.
			// A cancelled or timed-out context will fail every board in turn,
			// and reporting a dozen identical deadline errors buries the cause.
			if ctx.Err() != nil {
				break
			}

			continue
		}

		allJobs = append(allJobs, jobs...)
	}

	return allJobs, errors.Join(boardErrs...)
}
