package greenhouse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"jobclaw/internal/discovery"
)

func TestClientDiscover(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/v1/boards/example":
				fmt.Fprint(w, `{
					"name": "Example Company",
					"content": ""
				}`)

			case "/v1/boards/example/jobs":
				if r.URL.Query().Get("content") != "true" {
					t.Fatal("expected content=true")
				}

				fmt.Fprint(w, `{
					"jobs": [
						{
							"id": 12345,
							"title": "Backend Engineer",
							"absolute_url": "https://example.com/jobs/12345",
							"location": {
								"name": "Bangalore, India"
							},
							"content": "<p>Build backend systems.</p>"
						}
					]
				}`)

			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}),
	)
	defer server.Close()

	client := NewClient("example")
	client.baseURL = server.URL + "/v1"

	jobs, err := client.Discover(
		context.Background(),
		discovery.Request{},
	)
	if err != nil {
		t.Fatalf("discover jobs: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	got := jobs[0]

	if got.ExternalID != "12345" {
		t.Fatalf("unexpected external ID: %s", got.ExternalID)
	}

	if got.Company != "Example Company" {
		t.Fatalf("unexpected company: %s", got.Company)
	}

	if got.Title != "Backend Engineer" {
		t.Fatalf("unexpected title: %s", got.Title)
	}

	if got.Location != "Bangalore, India" {
		t.Fatalf("unexpected location: %s", got.Location)
	}

	if got.Description != "<p>Build backend systems.</p>" {
		t.Fatalf("unexpected description: %s", got.Description)
	}

	if got.URL != "https://example.com/jobs/12345" {
		t.Fatalf("unexpected URL: %s", got.URL)
	}

	if got.Source != "greenhouse" {
		t.Fatalf("unexpected source: %s", got.Source)
	}
}
