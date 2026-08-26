package application

import (
	"context"
	"errors"
	"testing"

	"jobclaw/internal/job"
)

func TestStaticGreenhouseFormProviderReturnsForm(
	t *testing.T,
) {
	form := GreenhouseApplicationForm{
		JobID: "gh-123",
		Fields: []GreenhouseFormField{
			{
				Name:     "work_authorization",
				Required: true,
			},
		},
	}

	provider, err := NewStaticGreenhouseFormProvider(form)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	got, err := provider.GetApplicationForm(
		context.Background(),
		job.Job{
			Source:     "greenhouse",
			ExternalID: "gh-123",
		},
	)
	if err != nil {
		t.Fatalf("get form: %v", err)
	}

	if got.JobID != form.JobID {
		t.Fatalf(
			"job ID = %q, want %q",
			got.JobID,
			form.JobID,
		)
	}

	if len(got.Fields) != 1 {
		t.Fatalf(
			"fields = %d, want 1",
			len(got.Fields),
		)
	}
}

func TestStaticGreenhouseFormProviderRejectsUnknownForm(
	t *testing.T,
) {
	provider, err := NewStaticGreenhouseFormProvider()
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = provider.GetApplicationForm(
		context.Background(),
		job.Job{
			Source:     "greenhouse",
			ExternalID: "missing",
		},
	)

	if err == nil {
		t.Fatal("expected missing form error")
	}
}

func TestStaticGreenhouseFormProviderRejectsDuplicateForm(
	t *testing.T,
) {
	_, err := NewStaticGreenhouseFormProvider(
		GreenhouseApplicationForm{JobID: "gh-123"},
		GreenhouseApplicationForm{JobID: "gh-123"},
	)

	if err == nil {
		t.Fatal("expected duplicate form error")
	}
}

func TestStaticGreenhouseFormProviderRespectsContext(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider, err := NewStaticGreenhouseFormProvider()
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	_, err = provider.GetApplicationForm(ctx, job.Job{
		Source:     "greenhouse",
		ExternalID: "gh-123",
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"error = %v, want context canceled",
			err,
		)
	}
}
