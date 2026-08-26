package greenhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		boardToken: strings.TrimSpace(boardToken),
	}
}

type boardResponse struct {
	Name string `json:"name"`
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

func (c *Client) Discover(
	ctx context.Context,
	request discovery.Request,
) ([]job.Job, error) {
	if c.boardToken == "" {
		return nil, fmt.Errorf("greenhouse board token is required")
	}

	company, err := c.fetchBoardName(ctx)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf(
			"decode greenhouse response: %w",
			err,
		)
	}

	result := make([]job.Job, 0, len(payload.Jobs))

	for _, item := range payload.Jobs {
		candidate := job.Job{
			Source:      "greenhouse",
			ExternalID:  fmt.Sprintf("%d", item.ID),
			Company:     company,
			Title:       item.Title,
			Description: item.Content,
			Location:    item.Location.Name,
			URL:         item.AbsoluteURL,
		}

		if !discovery.MatchesRequest(candidate, request) {
			continue
		}

		result = append(result, candidate)

		if request.Limit > 0 && len(result) >= request.Limit {
			break
		}
	}

	return result, nil
}

func (c *Client) fetchBoardName(
	ctx context.Context,
) (string, error) {
	url := fmt.Sprintf(
		"%s/boards/%s",
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
		return "", fmt.Errorf(
			"create greenhouse board request: %w",
			err,
		)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"request greenhouse board: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"greenhouse board returned HTTP %d",
			resp.StatusCode,
		)
	}

	var payload boardResponse

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf(
			"decode greenhouse board response: %w",
			err,
		)
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return "", fmt.Errorf(
			"greenhouse board %q has no company name",
			c.boardToken,
		)
	}

	return name, nil
}

func (c *Client) Name() string {
	return "greenhouse"
}
