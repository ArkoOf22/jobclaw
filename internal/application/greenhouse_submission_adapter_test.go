package application

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jobclaw/internal/job"
)

func TestGreenhouseSubmissionAdapterSubmitsPreparedData(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf(
					"method = %s, want POST",
					r.Method,
				)
			}

			if r.URL.Path != "/boards/acme/jobs/12345" {
				t.Fatalf(
					"path = %s, want /boards/acme/jobs/12345",
					r.URL.Path,
				)
			}

			username, password, ok := r.BasicAuth()

			if !ok {
				t.Fatal("expected Basic Auth")
			}

			if username != "test-api-key" {
				t.Fatalf(
					"username = %q, want test-api-key",
					username,
				)
			}

			if password != "" {
				t.Fatalf(
					"password = %q, want empty",
					password,
				)
			}

			contentType := r.Header.Get(
				"Content-Type",
			)

			if !strings.HasPrefix(
				contentType,
				"multipart/form-data;",
			) {
				t.Fatalf(
					"content type = %q, want multipart/form-data",
					contentType,
				)
			}

			if err := r.ParseMultipartForm(
				10 << 20,
			); err != nil {
				t.Fatalf(
					"parse multipart form: %v",
					err,
				)
			}

			if got := r.FormValue(
				"question_12345",
			); got != "1001" {
				t.Fatalf(
					"question value = %q, want 1001",
					got,
				)
			}

			file, header, err := r.FormFile(
				"resume",
			)
			if err != nil {
				t.Fatalf(
					"get resume file: %v",
					err,
				)
			}
			defer file.Close()

			if header.Filename != "resume.pdf" {
				t.Fatalf(
					"filename = %q, want resume.pdf",
					header.Filename,
				)
			}

			resume, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf(
					"read resume: %v",
					err,
				)
			}

			if string(resume) != "test resume" {
				t.Fatalf(
					"resume = %q, want test resume",
					string(resume),
				)
			}

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			fmt.Fprint(
				w,
				`{"id":"application-123"}`,
			)
		}),
	)
	defer server.Close()

	adapter := NewGreenhouseSubmissionAdapter(
		server.Client(),
		server.URL,
	)

	adapter.SetAPIKey(
		"test-api-key",
	)

	provider, err := NewStaticGreenhouseFormProvider(
		GreenhouseApplicationForm{
			JobID: "12345",
			Fields: []GreenhouseFormField{
				{
					ID:       "field-1",
					Name:     "question_12345",
					Required: true,
					Options: []GreenhouseFormOption{
						{
							Value: "1001",
							Label: "Yes",
						},
						{
							Value: "1002",
							Label: "No",
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"new form provider: %v",
			err,
		)
	}

	adapter.SetFormProvider(
		provider,
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Application: Application{
				ID:    100,
				JobID: 10,
			},
			Job: job.Job{
				ID:         10,
				Source:     "greenhouse",
				ExternalID: "12345",
				URL:        "https://job-boards.greenhouse.io/acme/jobs/12345",
			},
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "12345",
			},
			Prepared: PreparedSubmission{
				Resume: []byte(
					"test resume",
				),
				Answers: []ResolvedSubmissionAnswer{
					{
						FieldKey: "question_12345",
						Question: "Are you authorized to work?",
						Answer:   "Yes",
					},
				},
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"submit: %v",
			err,
		)
	}

	if result != SubmissionSucceeded {
		t.Fatalf(
			"result = %q, want %q",
			result,
			SubmissionSucceeded,
		)
	}
}

func TestGreenhouseSubmissionAdapterRejectsMissingAPIKey(
	t *testing.T,
) {
	adapter := NewGreenhouseSubmissionAdapter(
		&http.Client{},
		"http://example.com",
	)

	provider, err := NewStaticGreenhouseFormProvider(
		GreenhouseApplicationForm{
			JobID: "12345",
		},
	)
	if err != nil {
		t.Fatalf(
			"create form provider: %v",
			err,
		)
	}

	adapter.SetFormProvider(
		provider,
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Job: job.Job{
				URL:        "https://job-boards.greenhouse.io/acme/jobs/12345",
				ExternalID: "12345",
			},
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "12345",
			},
			Prepared: PreparedSubmission{
				Resume: []byte(
					"resume",
				),
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
		t.Fatal(
			"expected missing API key error",
		)
	}
}

func TestGreenhouseSubmissionAdapterRejectsHTTPFailure(
	t *testing.T,
) {
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

	adapter.SetAPIKey(
		"test-api-key",
	)

	provider, err := NewStaticGreenhouseFormProvider(
		GreenhouseApplicationForm{
			JobID: "12345",
		},
	)
	if err != nil {
		t.Fatalf(
			"create form provider: %v",
			err,
		)
	}

	adapter.SetFormProvider(
		provider,
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Job: job.Job{
				URL: "https://job-boards.greenhouse.io/acme/jobs/12345",
			},
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "12345",
			},
			Prepared: PreparedSubmission{
				Resume: []byte(
					"resume",
				),
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
		t.Fatal(
			"expected HTTP failure",
		)
	}
}

type greenhouseFailingRoundTripper struct{}

func (greenhouseFailingRoundTripper) RoundTrip(
	*http.Request,
) (*http.Response, error) {
	return nil, fmt.Errorf("simulated network failure")
}

func TestGreenhouseSubmissionAdapterNetworkFailureIsAmbiguous(
	t *testing.T,
) {
	adapter := NewGreenhouseSubmissionAdapter(
		&http.Client{
			Transport: greenhouseFailingRoundTripper{},
		},
		"http://greenhouse.test",
	)

	adapter.SetAPIKey(
		"test-api-key",
	)

	provider, err := NewStaticGreenhouseFormProvider(
		GreenhouseApplicationForm{
			JobID: "12345",
			Fields: []GreenhouseFormField{
				{
					ID:       "field-1",
					Name:     "question_12345",
					Label:    "Authorization",
					Type:     "input_text",
					Required: true,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"create form provider: %v",
			err,
		)
	}

	adapter.SetFormProvider(
		provider,
	)

	result, err := adapter.Submit(
		context.Background(),
		SubmissionRequest{
			Job: job.Job{
				URL:        "https://job-boards.greenhouse.io/acme/jobs/12345",
				ExternalID: "12345",
			},
			Target: SubmissionTarget{
				Type:       SubmissionTargetGreenhouse,
				ExternalID: "12345",
			},
			Prepared: PreparedSubmission{
				Resume: []byte(
					"resume",
				),
				Answers: []ResolvedSubmissionAnswer{
					{
						FieldKey: "question_12345",
						Question: "Authorization",
						Answer:   "Yes",
					},
				},
			},
		},
	)

	if result != SubmissionAmbiguous {
		t.Fatalf(
			"result = %q, want %q, err = %v",
			result,
			SubmissionAmbiguous,
			err,
		)
	}

	if err == nil {
		t.Fatal(
			"expected network error",
		)
	}
}
