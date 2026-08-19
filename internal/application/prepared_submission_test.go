package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jobclaw/internal/job"
)

func TestSubmissionDataProviderPrepareResolvesVerifiedAnswers(
	t *testing.T,
) {
	resumePath := filepath.Join(
		t.TempDir(),
		"resume.pdf",
	)

	if err := os.WriteFile(
		resumePath,
		[]byte("resume-content"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            1,
				ApplicationID: 100,
				Question:      "Are you authorized to work?",
				FieldKey:      "work_authorization",
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"work_authorization": {
				FieldKey: "work_authorization",
				Answer:   "Yes",
				Verified: true,
			},
		},
	}

	provider := NewSubmissionDataProvider(
		questions,
		NewAnswerResolver(answers),
	)

	prepared, err := provider.Prepare(
		context.Background(),
		Application{
			ID:                 100,
			JobID:              10,
			TailoredResumePath: resumePath,
		},
		job.Job{
			ID: 10,
		},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if string(prepared.Resume) != "resume-content" {
		t.Fatalf(
			"resume = %q, want %q",
			string(prepared.Resume),
			"resume-content",
		)
	}

	if len(prepared.Answers) != 1 {
		t.Fatalf(
			"answers = %d, want 1",
			len(prepared.Answers),
		)
	}

	if prepared.Answers[0].Answer != "Yes" {
		t.Fatalf(
			"answer = %q, want %q",
			prepared.Answers[0].Answer,
			"Yes",
		)
	}
}

func TestSubmissionDataProviderRejectsUnverifiedAnswer(
	t *testing.T,
) {
	resumePath := filepath.Join(
		t.TempDir(),
		"resume.pdf",
	)

	if err := os.WriteFile(
		resumePath,
		[]byte("resume"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            1,
				ApplicationID: 100,
				Question:      "Are you authorized to work?",
				FieldKey:      "work_authorization",
			},
		},
	}

	answers := &fakeAnswerRepository{
		answers: map[string]*CandidateAnswer{
			"work_authorization": {
				FieldKey: "work_authorization",
				Answer:   "Yes",
				Verified: false,
			},
		},
	}

	provider := NewSubmissionDataProvider(
		questions,
		NewAnswerResolver(answers),
	)

	_, err := provider.Prepare(
		context.Background(),
		Application{
			ID:                 100,
			JobID:              10,
			TailoredResumePath: resumePath,
		},
		job.Job{ID: 10},
	)

	if err == nil {
		t.Fatal("expected unverified answer error")
	}

	if !strings.Contains(err.Error(), "not verified") {
		t.Fatalf(
			"error = %v, want not verified",
			err,
		)
	}
}

func TestSubmissionDataProviderRejectsMissingAnswer(
	t *testing.T,
) {
	resumePath := filepath.Join(
		t.TempDir(),
		"resume.pdf",
	)

	if err := os.WriteFile(
		resumePath,
		[]byte("resume"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            1,
				ApplicationID: 100,
				Question:      "Are you authorized to work?",
				FieldKey:      "work_authorization",
			},
		},
	}

	provider := NewSubmissionDataProvider(
		questions,
		NewAnswerResolver(
			&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{},
			},
		),
	)

	_, err := provider.Prepare(
		context.Background(),
		Application{
			ID:                 100,
			JobID:              10,
			TailoredResumePath: resumePath,
		},
		job.Job{ID: 10},
	)

	if err == nil {
		t.Fatal("expected missing answer error")
	}

	if !strings.Contains(err.Error(), "no candidate answer") {
		t.Fatalf(
			"error = %v, want no candidate answer",
			err,
		)
	}
}

func TestSubmissionDataProviderAllowsNoQuestionnaire(
	t *testing.T,
) {
	resumePath := filepath.Join(
		t.TempDir(),
		"resume.pdf",
	)

	if err := os.WriteFile(
		resumePath,
		[]byte("resume"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	provider := NewSubmissionDataProvider(
		&fakeQuestionRepository{},
		NewAnswerResolver(
			&fakeAnswerRepository{},
		),
	)

	prepared, err := provider.Prepare(
		context.Background(),
		Application{
			ID:                 100,
			JobID:              10,
			TailoredResumePath: resumePath,
		},
		job.Job{ID: 10},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if len(prepared.Answers) != 0 {
		t.Fatalf(
			"answers = %d, want 0",
			len(prepared.Answers),
		)
	}
}

func TestSubmissionDataProviderRejectsMissingResume(
	t *testing.T,
) {
	provider := NewSubmissionDataProvider(
		&fakeQuestionRepository{},
		NewAnswerResolver(
			&fakeAnswerRepository{},
		),
	)

	_, err := provider.Prepare(
		context.Background(),
		Application{
			ID:     100,
			JobID:  10,
			Status: StatusReadyToApply,
		},
		job.Job{ID: 10},
	)

	if err == nil {
		t.Fatal("expected missing resume error")
	}

	if !strings.Contains(err.Error(), "tailored resume is missing") {
		t.Fatalf(
			"error = %v, want missing resume",
			err,
		)
	}
}
