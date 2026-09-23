// Package google implements the ResumeLLM contract against Google's
// Generative Language API (the AI Studio "Gemini API"), as a direct
// alternative to the OpenRouter client.
//
// PRIVACY, AND WHY THIS PACKAGE ASSUMES A PAID-TIER KEY.
//
// The OpenRouter client enforces two privacy guarantees per request:
// data_collection="deny" and zdr=true. It fails loudly if no provider
// satisfies them, because resume and questionnaire prompts embed the
// candidate's full employment history, education, and contact details.
//
// Google's Generative Language API has NO per-request equivalent of those
// flags. Whether prompts are used to improve Google's products is instead a
// property of the API tier tied to the key:
//
//   - Unpaid / free-tier AI Studio keys: prompts MAY be used for training.
//     This violates the deny-data-collection guarantee.
//   - Paid tier (billing enabled on the Cloud project behind the key):
//     Google states prompts are not used to train its models.
//
// This client therefore cannot replicate the "fail loudly if the provider
// retains data" behaviour in code, because the wire protocol offers nothing to
// assert against. The guarantee moves from a per-request flag to an
// operational precondition: the configured key MUST be paid tier. That
// assumption is made explicit here rather than papered over by pretending a
// ZDR flag still applies. RequireZeroDataRetention and DenyDataCollection are
// accepted for interface parity but only gate construction; they are not sent
// on the wire, because there is no field to send them in.
package google

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

// DefaultBaseURL is the v1beta Generative Language endpoint. The model name and
// the :generateContent method are appended per request.
const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type Config struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client

	// DenyDataCollection and RequireZeroDataRetention are accepted for parity
	// with the OpenRouter client so the composition root can pass the same
	// privacy config to either provider. Google's API has no per-request flag
	// for either, so these are NOT transmitted. See the package comment: the
	// privacy guarantee here is the paid-tier precondition on APIKey, not a
	// wire-level assertion. They are retained so a future caller can choose to
	// treat them as a hard construction-time requirement if desired.
	DenyDataCollection       bool
	RequireZeroDataRetention bool
}

type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("Google API key is required")
	}

	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("Google model is required")
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
		apiKey:     cfg.APIKey,
		model:      strings.TrimSpace(cfg.Model),
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

type generateContentRequest struct {
	Contents []content `json:"contents"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`

	// PromptFeedback carries a block reason when the prompt itself was
	// refused, in which case Candidates is empty.
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback,omitempty"`

	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
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

	payload := generateContentRequest{
		Contents: []content{
			{
				Role: "user",
				Parts: []part{
					{Text: prompt},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode Google request: %w", err)
	}

	// Model and method live in the path: /models/{model}:generateContent.
	url := fmt.Sprintf(
		"%s/models/%s:generateContent",
		c.baseURL,
		c.model,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create Google request: %w", err)
	}

	// The key travels in a header rather than the query string so it is not
	// captured in access logs or error messages that echo the URL.
	req.Header.Set("x-goog-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Google request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read Google response: %w", err)
	}

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf(
			"Google returned HTTP %d: %s",
			resp.StatusCode,
			sanitizeErrorBody(responseBody),
		)
	}

	var response generateContentResponse

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("decode Google response: %w", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf(
			"Google error: %s",
			response.Error.Message,
		)
	}

	if response.PromptFeedback != nil &&
		strings.TrimSpace(response.PromptFeedback.BlockReason) != "" {
		return "", fmt.Errorf(
			"Google blocked the prompt: %s",
			response.PromptFeedback.BlockReason,
		)
	}

	if len(response.Candidates) == 0 {
		return "", fmt.Errorf("Google response contains no candidates")
	}

	var builder strings.Builder

	for _, candidatePart := range response.Candidates[0].Content.Parts {
		builder.WriteString(candidatePart.Text)
	}

	result := strings.TrimSpace(builder.String())

	if result == "" {
		return "", fmt.Errorf("Google response content is empty")
	}

	return result, nil
}

// sanitizeErrorBody bounds an upstream error body and never returns empty, so a
// non-2xx status always carries some diagnostic text. It is applied to a
// response body that never contains the API key.
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
