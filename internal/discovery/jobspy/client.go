package jobspy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jobclaw/internal/discovery"
	"jobclaw/internal/job"
)

const defaultEndpoint = "http://127.0.0.1:8000/mcp"

// Compile-time verification that Client implements discovery.Source.
var _ discovery.Source = (*Client)(nil)

type Client struct {
	endpoint   string
	httpClient *http.Client
}

func NewClient(endpoint string) *Client {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultEndpoint
	}

	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *Client) Name() string {
	return "jobspy"
}

func (c *Client) Discover(ctx context.Context, req discovery.Request) ([]job.Job, error) {
	searchTerm := strings.TrimSpace(strings.Join(req.Keywords, " "))
	if searchTerm == "" {
		return nil, fmt.Errorf("at least one keyword is required")
	}

	location := strings.TrimSpace(strings.Join(req.Locations, ", "))

	args := searchJobsRequest{
		SearchTerm:         searchTerm,
		Sites:              []string{"linkedin", "naukri", "indeed"},
		Location:           location,
		ResultsWanted:      req.Limit,
		IsRemote:           req.RemoteOnly,
		HoursOld:           req.HoursOld,
		IncludeDescription: true,
	}

	if args.ResultsWanted <= 0 {
		args.ResultsWanted = 20
	}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "jobclaw",
			Version: "0.1.0",
		},
		nil,
	)

	transport := &mcp.StreamableClientTransport{
		Endpoint:             c.endpoint,
		HTTPClient:           c.httpClient,
		DisableStandaloneSSE: true,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to JobSpy MCP: %w", err)
	}
	defer session.Close()

	params := &mcp.CallToolParams{
		Name:      "search_jobs",
		Arguments: args,
	}

	result, err := session.CallTool(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("call search_jobs: %w", err)
	}

	if result.IsError {
		return nil, fmt.Errorf("JobSpy returned a tool error")
	}

	response, err := decodeSearchResponse(result)
	if err != nil {
		return nil, fmt.Errorf("decode JobSpy response: %w", err)
	}

	return normalizeJobs(response.Jobs), nil
}

func decodeSearchResponse(result *mcp.CallToolResult) (searchJobsResponse, error) {
	var response searchJobsResponse

	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}

		if err := json.Unmarshal([]byte(text.Text), &response); err == nil {
			return response, nil
		}
	}

	return searchJobsResponse{}, fmt.Errorf("no valid JSON response in tool result")
}

func normalizeJobs(rawJobs []rawJob) []job.Job {
	now := time.Now().UTC()
	jobs := make([]job.Job, 0, len(rawJobs))

	for _, raw := range rawJobs {
		if strings.TrimSpace(raw.ID) == "" ||
			strings.TrimSpace(raw.Company) == "" ||
			strings.TrimSpace(raw.Title) == "" ||
			strings.TrimSpace(raw.JobURL) == "" {
			continue
		}

		postedAt := parsePostedAt(raw.DatePosted)

		remoteType := "ONSITE"
		if raw.IsRemote {
			remoteType = "REMOTE"
		}

		externalID := raw.ID
		if raw.Site != "" {
			externalID = raw.Site + ":" + raw.ID
		}

		jobs = append(jobs, job.Job{
			Source:         raw.Site,
			ExternalID:     externalID,
			Company:        raw.Company,
			Title:          raw.Title,
			Description:    raw.Description,
			Location:       raw.Location,
			RemoteType:     remoteType,
			EmploymentType: raw.JobType,
			URL:            raw.JobURL,
			PostedAt:       postedAt,
			DiscoveredAt:   now,
		})
	}

	return jobs
}

func parsePostedAt(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}

	t = t.UTC()
	return &t
}
