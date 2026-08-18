package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMockResumeGeneratorCreatesArtifact(t *testing.T) {
	generator := NewMockResumeGenerator()

	outputPath := filepath.Join(
		t.TempDir(),
		"resume",
		"tailored_resume.txt",
	)

	err := generator.GenerateTailoredResume(
		context.Background(),
		7,
		outputPath,
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	if !strings.Contains(content, "JobClaw Tailored Resume") {
		t.Fatalf("unexpected resume content: %q", content)
	}

	if !strings.Contains(content, "Job ID: 7") {
		t.Fatalf("resume does not contain job ID: %q", content)
	}
}

func TestMockResumeGeneratorRejectsInvalidInput(t *testing.T) {
	generator := NewMockResumeGenerator()

	tests := []struct {
		name       string
		jobID      int64
		outputPath string
	}{
		{
			name:       "invalid job ID",
			jobID:      0,
			outputPath: "resume.txt",
		},
		{
			name:       "missing output path",
			jobID:      7,
			outputPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generator.GenerateTailoredResume(
				context.Background(),
				tt.jobID,
				tt.outputPath,
			)

			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMockResumeGeneratorHonorsCancelledContext(t *testing.T) {
	generator := NewMockResumeGenerator()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := generator.GenerateTailoredResume(
		ctx,
		7,
		filepath.Join(t.TempDir(), "resume.txt"),
	)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("unexpected error: %v", err)
	}
}
