package greenhouse

import (
	"context"
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

func (c *MultiBoardClient) Discover(
	ctx context.Context,
	request discovery.Request,
) ([]job.Job, error) {
	if len(c.clients) == 0 {
		return nil, fmt.Errorf("no greenhouse board tokens configured")
	}

	allJobs := make([]job.Job, 0)

	for _, client := range c.clients {
		jobs, err := client.Discover(ctx, request)
		if err != nil {
			return nil, err
		}

		allJobs = append(allJobs, jobs...)
	}

	return allJobs, nil
}
