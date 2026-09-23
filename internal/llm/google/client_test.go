package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGenerate(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}

			if r.URL.Path != "/models/test-model:generateContent" {
				t.Fatalf(
					"path = %s, want /models/test-model:generateContent",
					r.URL.Path,
				)
			}

			if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
				t.Fatalf("x-goog-api-key = %q, want test-key", got)
			}

			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type = %q", got)
			}

			var request generateContentRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			if len(request.Contents) != 1 {
				t.Fatalf("contents = %d, want 1", len(request.Contents))
			}

			if request.Contents[0].Role != "user" {
				t.Fatalf("role = %q, want user", request.Contents[0].Role)
			}

			if len(request.Contents[0].Parts) != 1 {
				t.Fatalf(
					"parts = %d, want 1",
					len(request.Contents[0].Parts),
				)
			}

			if request.Contents[0].Parts[0].Text != "tailor this resume" {
				t.Fatalf(
					"text = %q",
					request.Contents[0].Parts[0].Text,
				)
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"candidates": [
					{
						"content": {
							"parts": [
								{ "text": "Tailored resume content" }
							]
						},
						"finishReason": "STOP"
					}
				]
			}`))
		}),
	)
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Generate(
		context.Background(),
		"tailor this resume",
	)
	if err != nil {
		t.Fatal(err)
	}

	if result != "Tailored resume content" {
		t.Fatalf("result = %q, want %q", result, "Tailored resume content")
	}
}

func TestClientGenerateJoinsMultipleParts(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"candidates": [
					{
						"content": {
							"parts": [
								{ "text": "Hello " },
								{ "text": "world" }
							]
						}
					}
				]
			}`))
		}),
	)
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}

	if result != "Hello world" {
		t.Fatalf("result = %q, want %q", result, "Hello world")
	}
}

func TestNewClientRejectsMissingConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "missing API key",
			config: Config{
				Model: "test-model",
			},
		},
		{
			name: "missing model",
			config: Config{
				APIKey: "test-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewClient(tt.config); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestClientGenerateRejectsEmptyPrompt(t *testing.T) {
	client, err := NewClient(Config{
		APIKey: "test-key",
		Model:  "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Generate(
		context.Background(),
		"   ",
	); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientGenerateHandlesHTTPError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(
				w,
				`{"error":{"code":400,"message":"API key not valid","status":"INVALID_ARGUMENT"}}`,
				http.StatusBadRequest,
			)
		}),
	)
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(err.Error(), "test-key") {
		t.Fatal("API key leaked in error")
	}
}

func TestClientGenerateHandlesEmptyCandidates(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{ "candidates": [] }`))
		}),
	)
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "no candidates") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGenerateHandlesBlockedPrompt(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"promptFeedback": { "blockReason": "SAFETY" }
			}`))
		}),
	)
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "SAFETY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGenerateHandlesEmptyContent(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"candidates": [
					{
						"content": {
							"parts": [
								{ "text": "   " }
							]
						}
					}
				]
			}`))
		}),
	)
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "content is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGenerateHonorsCancelledContext(t *testing.T) {
	client, err := NewClient(Config{
		APIKey: "test-key",
		Model:  "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Generate(ctx, "test prompt"); err == nil {
		t.Fatal("expected cancellation error")
	}
}
