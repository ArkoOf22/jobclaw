package application

import (
	"context"
	"errors"
	"os"
	"testing"

	"jobclaw/internal/job"
)

type fakeSubmissionAdapter struct {
	result  SubmissionResult
	err     error
	calls   int
	request SubmissionRequest
}

func (f *fakeSubmissionAdapter) Submit(
	ctx context.Context,
	request SubmissionRequest,
) (SubmissionResult, error) {
	f.calls++
	f.request = request

	if f.err != nil {
		return f.result, f.err
	}

	if f.result == "" {
		return SubmissionSucceeded, nil
	}

	return f.result, nil
}

func TestStaticSubmissionAdapterRegistryRegisterAndGet(t *testing.T) {
	registry := NewStaticSubmissionAdapterRegistry()
	adapter := &fakeSubmissionAdapter{}

	if err := registry.Register(
		SubmissionTargetGreenhouse,
		adapter,
	); err != nil {
		t.Fatalf("register adapter: %v", err)
	}

	got, err := registry.Get(SubmissionTargetGreenhouse)
	if err != nil {
		t.Fatalf("get adapter: %v", err)
	}

	if got != adapter {
		t.Fatal("registry returned a different adapter")
	}
}

func TestStaticSubmissionAdapterRegistryRejectsMissingAdapter(
	t *testing.T,
) {
	registry := NewStaticSubmissionAdapterRegistry()

	err := registry.Register(
		SubmissionTargetGreenhouse,
		nil,
	)
	if err == nil {
		t.Fatal("expected missing adapter error")
	}
}

func TestStaticSubmissionAdapterRegistryRejectsUnknownTarget(
	t *testing.T,
) {
	registry := NewStaticSubmissionAdapterRegistry()

	_, err := registry.Get(SubmissionTargetGreenhouse)
	if err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestRegistrySubmissionSubmitterRoutesToAdapter(t *testing.T) {
	registry := NewStaticSubmissionAdapterRegistry()

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{},
	}

	answers := &fakeAnswerRepository{}

	resumePath := t.TempDir() + "/resume.pdf"
	if err := os.WriteFile(
		resumePath,
		[]byte("test resume"),
		0600,
	); err != nil {
		t.Fatalf("write test resume: %v", err)
	}

	adapter := &fakeSubmissionAdapter{
		result: SubmissionSucceeded,
	}

	if err := registry.Register(
		SubmissionTargetGreenhouse,
		adapter,
	); err != nil {
		t.Fatalf("register adapter: %v", err)
	}

	submitter := NewRegistrySubmissionSubmitter(
		registry,
	)
	submitter.SetDataProvider(
		questions,
		NewAnswerResolver(answers),
	)

	app := Application{
		ID:                 100,
		JobID:              10,
		TailoredResumePath: resumePath,
	}

	j := job.Job{
		ID:         10,
		Source:     "greenhouse",
		ExternalID: "gh-123",
		URL:        "https://example.com/jobs/123",
	}

	result, err := submitter.Submit(
		context.Background(),
		app,
		j,
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

	if adapter.calls != 1 {
		t.Fatalf(
			"adapter calls = %d, want 1",
			adapter.calls,
		)
	}

	if adapter.request.Application.ID != app.ID {
		t.Fatalf("application ID = %d, want %d",
			adapter.request.Application.ID,
			app.ID,
		)
	}

	if adapter.request.Job.ID != j.ID {
		t.Fatalf(
			"job ID = %d, want %d",
			adapter.request.Job.ID,
			j.ID,
		)
	}

	if adapter.request.Target.Type != SubmissionTargetGreenhouse {
		t.Fatalf(
			"target type = %q, want %q",
			adapter.request.Target.Type,
			SubmissionTargetGreenhouse,
		)
	}

	if adapter.request.Target.ExternalID != "gh-123" {
		t.Fatalf(
			"external ID = %q, want %q",
			adapter.request.Target.ExternalID,
			"gh-123",
		)
	}

	if string(adapter.request.Prepared.Resume) != "test resume" {
		t.Fatalf(
			"prepared resume = %q, want %q",
			string(adapter.request.Prepared.Resume),
			"test resume",
		)
	}

	if adapter.request.Prepared.Application.ID != app.ID {
		t.Fatalf(
			"prepared application ID = %d, want %d",
			adapter.request.Prepared.Application.ID,
			app.ID,
		)
	}

	if adapter.request.Prepared.Job.ID != j.ID {
		t.Fatalf(
			"prepared job ID = %d, want %d",
			adapter.request.Prepared.Job.ID,
			j.ID,
		)
	}

	if len(adapter.request.Prepared.Answers) != 0 {
		t.Fatalf(
			"prepared answers = %d, want 0",
			len(adapter.request.Prepared.Answers),
		)
	}
}

func TestRegistrySubmissionSubmitterPropagatesAdapterResult(
	t *testing.T,
) {
	registry := NewStaticSubmissionAdapterRegistry()

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{},
	}

	answers := &fakeAnswerRepository{}

	resumePath := t.TempDir() + "/resume.pdf"
	if err := os.WriteFile(
		resumePath,
		[]byte("test resume"),
		0600,
	); err != nil {
		t.Fatalf("write test resume: %v", err)
	}

	adapterErr := errors.New("adapter failed")

	adapter := &fakeSubmissionAdapter{
		result: SubmissionAmbiguous,
		err:    adapterErr,
	}

	if err := registry.Register(
		SubmissionTargetGeneric,
		adapter,
	); err != nil {
		t.Fatalf("register adapter: %v", err)
	}

	submitter := NewRegistrySubmissionSubmitter(
		registry,
	)
	submitter.SetDataProvider(
		questions,
		NewAnswerResolver(answers),
	)

	result, err := submitter.Submit(
		context.Background(),
		Application{
			ID:                 100,
			JobID:              10,
			TailoredResumePath: resumePath,
		},
		job.Job{ID: 10},
	)

	if !errors.Is(err, adapterErr) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			adapterErr,
		)
	}

	if result != SubmissionAmbiguous {
		t.Fatalf(
			"result = %q, want %q",
			result,
			SubmissionAmbiguous,
		)
	}
}
