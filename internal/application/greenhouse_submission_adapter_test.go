package application

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"jobclaw/internal/job"
)

func TestGreenhouseSubmissionAdapterSubmitsPreparedData(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}

			if r.URL.Path != "/submit" {
				t.Fatalf("path = %s, want /submit", r.URL.Path)
			}

			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf(
					"content type = %q, want application/json",
					got,
				)
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"success": true,
				"id": "submission-123"
			}`)
		}),
	)
	defer server.Close()

	adapter := NewGreenhouseSubmissionAdapter(
		server.Client(),
		server.URL,
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Application: Application{ID: 100, JobID: 10},
			Job: job.Job{
				ID:         10,
				Source:     "greenhouse",
				ExternalID: "gh-123",
			},
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "gh-123",
			},
			Prepared: PreparedSubmission{
				Resume: []byte("test resume"),
				Answers: []ResolvedSubmissionAnswer{
					{
						FieldKey: "work_authorization",
						Question: "Are you authorized to work?",
						Answer:   "Yes",
					},
				},
			},
		},
	)

	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if result != SubmissionSucceeded {
		t.Fatalf(
			"result = %q, want %q",
			result,
			SubmissionSucceeded,
		)
	}
}

func TestGreenhouseSubmissionAdapterRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(
				w,
				"invalid application",
				http.StatusBadRequest,
			)
		}),
	)
	defer server.Close()

	adapter := NewGreenhouseSubmissionAdapter(
		server.Client(),
		server.URL,
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "gh-123",
			},
			Prepared: PreparedSubmission{
				Resume: []byte("resume"),
			},
		},
	)

	if result != SubmissionFailed {
		t.Fatalf(
			"result = %q, want %q",
			result,
			SubmissionFailed,
		)
	}

	if err == nil {
		t.Fatal("expected HTTP failure")
	}
}

func TestGreenhouseSubmissionAdapterNetworkFailureIsAmbiguous(t *testing.T) {
	adapter := NewGreenhouseSubmissionAdapter(
		&http.Client{},
		"http://127.0.0.1:1",
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "gh-123",
			},
			Prepared: PreparedSubmission{
				Resume: []byte("resume"),
			},
		},
	)

	if result != SubmissionAmbiguous {
		t.Fatalf(
			"result = %q, want %q",
			result,
			SubmissionAmbiguous,
		)
	}

	if err == nil {
		t.Fatal("expected network error")
	}
}
