package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jobclaw/internal/job"
)

func TestResumePromptBuilderBuildsPrompt(t *testing.T) {
	dir := t.TempDir()
	resumePath := filepath.Join(dir, "master_resume.txt")

	masterResume := `Arkodeep Koley

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Worked with Kafka.
- Reduced log volume by 92%.
`

	if err := os.WriteFile(
		resumePath,
		[]byte(masterResume),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: resumePath,
	})

	builder := NewResumePromptBuilder(source)

	j := &job.Job{
		ID:          7,
		Company:     "Setu",
		Title:       "SDE II",
		Location:    "Bengaluru",
		Description: "Build scalable backend systems using Go and distributed systems.",
	}

	prompt, err := builder.Build(j)
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		"JobClaw's resume tailoring engine",
		"MASTER RESUME is the only factual source of truth",
		"Do not invent experience.",
		"Never invent, estimate, infer, calculate, derive, round, or extrapolate a metric.",
		"Setu",
		"SDE II",
		"Bengaluru",
		"Build scalable backend systems using Go and distributed systems.",
		"Arkodeep Koley",
		"Built Go microservices.",
		"Reduced log volume by 92%.",
		"Return only the tailored resume.",
	}

	for _, expected := range checks {
		if !strings.Contains(prompt, expected) {
			t.Fatalf(
				"prompt missing %q:\n%s",
				expected,
				prompt,
			)
		}
	}
}

func TestResumePromptBuilderRejectsInvalidJob(t *testing.T) {
	dir := t.TempDir()
	resumePath := filepath.Join(dir, "master_resume.txt")

	if err := os.WriteFile(
		resumePath,
		[]byte("Master Resume"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: resumePath,
	})

	builder := NewResumePromptBuilder(source)

	tests := []struct {
		name string
		job  *job.Job
	}{
		{
			name: "nil job",
			job:  nil,
		},
		{
			name: "invalid job ID",
			job: &job.Job{
				ID:          0,
				Title:       "Backend Engineer",
				Description: "Backend systems",
			},
		},
		{
			name: "missing title",
			job: &job.Job{
				ID:          1,
				Description: "Backend systems",
			},
		},
		{
			name: "missing description",
			job: &job.Job{
				ID:    1,
				Title: "Backend Engineer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := builder.Build(tt.job); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
