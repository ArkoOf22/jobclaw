package openrouter

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

			if r.URL.Path != "/api/v1/chat/completions" {
				t.Fatalf(
					"path = %s, want /api/v1/chat/completions",
					r.URL.Path,
				)
			}

			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization = %q", got)
			}

			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type = %q", got)
			}

			var request chatRequest

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}

			if request.Model != "test-model" {
				t.Fatalf(
					"model = %q, want %q",
					request.Model,
					"test-model",
				)
			}

			if len(request.Messages) != 1 {
				t.Fatalf(
					"messages = %d, want 1",
					len(request.Messages),
				)
			}

			if request.Messages[0].Role != "user" {
				t.Fatalf(
					"role = %q, want user",
					request.Messages[0].Role,
				)
			}

			if request.Messages[0].Content != "tailor this resume" {
				t.Fatalf(
					"content = %q",
					request.Messages[0].Content,
				)
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"content": "Tailored resume content"
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
		BaseURL: server.URL + "/api/v1",
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
		t.Fatalf(
			"result = %q, want %q",
			result,
			"Tailored resume content",
		)
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
				`{"error":{"message":"invalid API key"}}`,
				http.StatusUnauthorized,
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

	_, err = client.Generate(
		context.Background(),
		"test prompt",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(err.Error(), "invalid API key") {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(err.Error(), "test-key") {
		t.Fatal("API key leaked in error")
	}
}

func TestClientGenerateHandlesEmptyChoices(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"choices": []
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

	_, err = client.Generate(
		context.Background(),
		"test prompt",
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientGenerateHandlesEmptyContent(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"choices": [
					{
						"message": {
							"content": "   "
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

	_, err = client.Generate(
		context.Background(),
		"test prompt",
	)
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

	_, err = client.Generate(ctx, "test prompt")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestClientGenerateSendsProviderPrivacyPreferences(t *testing.T) {
	testCases := []struct {
		name                     string
		denyDataCollection       bool
		requireZeroDataRetention bool
		wantProvider             bool
		wantDataCollection       string
		wantZDR                  bool
	}{
		{
			name:                     "omitted when unrestricted",
			denyDataCollection:       false,
			requireZeroDataRetention: false,
			wantProvider:             false,
		},
		{
			name:                     "deny data collection only",
			denyDataCollection:       true,
			requireZeroDataRetention: false,
			wantProvider:             true,
			wantDataCollection:       "deny",
			wantZDR:                  false,
		},
		{
			name:                     "zero data retention only",
			denyDataCollection:       false,
			requireZeroDataRetention: true,
			wantProvider:             true,
			wantDataCollection:       "",
			wantZDR:                  true,
		},
		{
			name:                     "both restrictions",
			denyDataCollection:       true,
			requireZeroDataRetention: true,
			wantProvider:             true,
			wantDataCollection:       "deny",
			wantZDR:                  true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var request chatRequest

			server := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						if err := json.NewDecoder(r.Body).
							Decode(&request); err != nil {
							t.Fatalf("decode request: %v", err)
						}

						w.Header().Set(
							"Content-Type",
							"application/json",
						)
						_, _ = w.Write([]byte(
							`{"choices":[{"message":{"content":"ok"}}]}`,
						))
					},
				),
			)
			defer server.Close()

			client, err := NewClient(
				Config{
					APIKey:                   "test-key",
					Model:                    "test-model",
					BaseURL:                  server.URL + "/api/v1",
					DenyDataCollection:       testCase.denyDataCollection,
					RequireZeroDataRetention: testCase.requireZeroDataRetention,
				},
			)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}

			if _, err := client.Generate(
				context.Background(),
				"tailor this resume",
			); err != nil {
				t.Fatalf("generate: %v", err)
			}

			if !testCase.wantProvider {
				if request.Provider != nil {
					t.Fatalf(
						"provider = %+v, want omitted",
						request.Provider,
					)
				}

				return
			}

			if request.Provider == nil {
				t.Fatal("provider is nil, want preferences sent")
			}

			if request.Provider.DataCollection !=
				testCase.wantDataCollection {
				t.Fatalf(
					"data_collection = %q, want %q",
					request.Provider.DataCollection,
					testCase.wantDataCollection,
				)
			}

			if request.Provider.ZDR != testCase.wantZDR {
				t.Fatalf(
					"zdr = %v, want %v",
					request.Provider.ZDR,
					testCase.wantZDR,
				)
			}
		})
	}
}
