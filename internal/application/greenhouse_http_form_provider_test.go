package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jobclaw/internal/job"
)

func TestGreenhouseHTTPFormProviderReturnsForm(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", r.Method)
			}

			if r.URL.Path != "/boards/acme/jobs/12345" {
				t.Fatalf(
					"path = %s, want /boards/acme/jobs/12345",
					r.URL.Path,
				)
			}

			if r.URL.Query().Get("questions") != "true" {
				t.Fatalf(
					"questions = %q, want true",
					r.URL.Query().Get("questions"),
				)
			}

			w.Header().Set("Content-Type", "application/json")

			_, _ = w.Write([]byte(`{
				"id": 12345,
				"questions": [
					{
						"label": "Are you authorized to work?",
						"required": true,
						"fields": [
							{
								"id": "field_12345",
								"name": "question_12345",
								"type": "multi_value_single_select",
								"values": [
									{
										"value": 1001,
										"label": "Yes"
									},
									{
										"value": 1002,
										"label": "No"
									}
								]
							}
						]
					},
					{
						"label": "LinkedIn URL",
						"required": false,
						"fields": [
							{
								"id": "field_67890",
								"name": "question_67890",
								"type": "input_text",
								"values": []
							}
						]
					}
				]
			}`))
		}),
	)
	defer server.Close()

	provider := NewGreenhouseHTTPFormProvider(
		server.Client(),
		server.URL,
	)

	form, err := provider.GetApplicationForm(
		context.Background(),
		job.Job{
			Source:     "greenhouse",
			ExternalID: "12345",
			URL:        "https://job-boards.greenhouse.io/acme/jobs/12345",
		},
	)
	if err != nil {
		t.Fatalf("get application form: %v", err)
	}

	if form.JobID != "12345" {
		t.Fatalf(
			"job ID = %q, want %q",
			form.JobID,
			"12345",
		)
	}

	if len(form.Fields) != 2 {
		t.Fatalf(
			"fields = %d, want 2",
			len(form.Fields),
		)
	}

	field := form.Fields[0]

	if field.ID != "field_12345" {
		t.Fatalf(
			"field ID = %q, want field_12345",
			field.ID,
		)
	}

	if field.Name != "question_12345" {
		t.Fatalf(
			"field name = %q, want question_12345",
			field.Name,
		)
	}

	if field.Label != "Are you authorized to work?" {
		t.Fatalf(
			"field label = %q, want authorization question",
			field.Label,
		)
	}

	if field.Type != "multi_value_single_select" {
		t.Fatalf(
			"field type = %q, want multi_value_single_select",
			field.Type,
		)
	}

	if !field.Required {
		t.Fatal("field should be required")
	}

	if len(field.Options) != 2 {
		t.Fatalf(
			"options = %d, want 2",
			len(field.Options),
		)
	}

	if field.Options[0].Value != "1001" {
		t.Fatalf(
			"option value = %q, want 1001",
			field.Options[0].Value,
		)
	}

	if field.Options[0].Label != "Yes" {
		t.Fatalf(
			"option label = %q, want Yes",
			field.Options[0].Label,
		)
	}
}

func TestGreenhouseHTTPFormProviderRejectsInvalidJobURL(
	t *testing.T,
) {
	provider := NewGreenhouseHTTPFormProvider(
		&http.Client{},
		"http://example.com",
	)

	_, err := provider.GetApplicationForm(
		context.Background(),
		job.Job{
			Source:     "greenhouse",
			ExternalID: "12345",
			URL:        "https://example.com/jobs/12345",
		},
	)

	if err == nil {
		t.Fatal("expected invalid greenhouse URL error")
	}
}

func TestGreenhouseHTTPFormProviderRejectsHTTPFailure(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(
				w,
				"not found",
				http.StatusNotFound,
			)
		}),
	)
	defer server.Close()

	provider := NewGreenhouseHTTPFormProvider(
		server.Client(),
		server.URL,
	)

	_, err := provider.GetApplicationForm(
		context.Background(),
		job.Job{
			Source:     "greenhouse",
			ExternalID: "12345",
			URL:        "https://job-boards.greenhouse.io/acme/jobs/12345",
		},
	)

	if err == nil {
		t.Fatal("expected HTTP failure")
	}
}
