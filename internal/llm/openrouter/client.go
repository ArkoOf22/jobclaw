package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "https://openrouter.ai/api/v1"

type Config struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client

	// DenyDataCollection restricts routing to providers that do not store
	// prompt data. Prompts here carry candidate PII, so callers should
	// normally enable this.
	DenyDataCollection bool

	// RequireZeroDataRetention restricts routing to Zero Data Retention
	// endpoints.
	RequireZeroDataRetention bool
}

type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client

	denyDataCollection       bool
	requireZeroDataRetention bool
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("OpenRouter API key is required")
	}

	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("OpenRouter model is required")
	}

	baseURL := strings.TrimRight(
		strings.TrimSpace(cfg.BaseURL),
		"/",
	)

	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 60 * time.Second,
		}
	}

	return &Client{
		apiKey:                   cfg.APIKey,
		model:                    cfg.Model,
		baseURL:                  baseURL,
		httpClient:               httpClient,
		denyDataCollection:       cfg.DenyDataCollection,
		requireZeroDataRetention: cfg.RequireZeroDataRetention,
	}, nil
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`

	// Provider carries OpenRouter routing preferences. Omitted entirely when
	// no restrictions are configured, so the wire format is unchanged for
	// callers that do not opt in.
	Provider *providerPreferences `json:"provider,omitempty"`
}

// providerPreferences mirrors OpenRouter's provider routing controls.
// See https://openrouter.ai/docs/guides/routing/provider-selection
type providerPreferences struct {
	// DataCollection is "deny" to permit only providers that do not store
	// user data. OpenRouter's default is "allow".
	DataCollection string `json:"data_collection,omitempty"`

	// ZDR restricts routing to Zero Data Retention endpoints.
	ZDR bool `json:"zdr,omitempty"`
}

// buildProviderPreferences returns nil when nothing is restricted, so the
// "provider" key is omitted from the request body entirely.
func (c *Client) buildProviderPreferences() *providerPreferences {
	if !c.denyDataCollection && !c.requireZeroDataRetention {
		return nil
	}

	preferences := &providerPreferences{
		ZDR: c.requireZeroDataRetention,
	}

	if c.denyDataCollection {
		preferences.DataCollection = "deny"
	}

	return preferences
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Generate(
	ctx context.Context,
	prompt string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}

	payload := chatRequest{
		Model: c.model,
		Messages: []message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Provider: c.buildProviderPreferences(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode OpenRouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create OpenRouter request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenRouter request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read OpenRouter response: %w", err)
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf(
			"OpenRouter returned HTTP %d: %s",
			resp.StatusCode,
			sanitizeErrorBody(responseBody),
		)
	}

	var response chatResponse

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("decode OpenRouter response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf(
			"OpenRouter error: %s",
			response.Error.Message,
		)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter response contains no choices")
	}

	result := strings.TrimSpace(
		response.Choices[0].Message.Content,
	)

	if result == "" {
		return "", fmt.Errorf("OpenRouter response content is empty")
	}

	return result, nil
}

func sanitizeErrorBody(body []byte) string {
	const maxLength = 500

	text := strings.TrimSpace(string(body))

	if len(text) > maxLength {
		text = text[:maxLength] + "..."
	}

	if text == "" {
		return "empty response"
	}

	return text
}
