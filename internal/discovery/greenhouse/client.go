package greenhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"jobclaw/internal/discovery"
	"jobclaw/internal/job"
)

const defaultBaseURL = "https://boards-api.greenhouse.io/v1"

type Client struct {
	httpClient *http.Client
	baseURL    string
	boardToken string
}

func NewClient(boardToken string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		baseURL:    defaultBaseURL,
		boardToken: boardToken,
	}
}

type jobsResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	AbsoluteURL string `json:"absolute_url"`
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
	Content string `json:"content"`
}

func (c *Client) Discover(ctx context.Context, request discovery.Request) ([]job.Job, error) {
	if c.boardToken == "" {
		return nil, fmt.Errorf("greenhouse board token is required")
	}

	url := fmt.Sprintf(
		"%s/boards/%s/jobs?content=true",
		c.baseURL,
		c.boardToken,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create greenhouse request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request greenhouse jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"greenhouse returned HTTP %d",
			resp.StatusCode,
		)
	}

	var payload jobsResponse

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode greenhouse response: %w", err)
	}

	result := make([]job.Job, 0, len(payload.Jobs))

	for _, item := range payload.Jobs {
		result = append(result, job.Job{
			Source:      "greenhouse",
			ExternalID:  fmt.Sprintf("%d", item.ID),
			Title:       item.Title,
			Description: item.Content,
			Location:    item.Location.Name,
			URL:         item.AbsoluteURL,
		})
	}

	return result, nil
}

func (c *Client) Name() string {
	return "greenhouse"
}
