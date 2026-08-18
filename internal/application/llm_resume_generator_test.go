package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

func setupLLMResumeGeneratorTest(t *testing.T) (
	*database.DB,
	*job.SQLiteRepository,
	*LLMResumeGenerator,
) {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if err := db.MigrateEmbedded(); err != nil {
		db.Close()
		t.Fatal(err)
	}

	jobRepo := job.NewSQLiteRepository(db)

	if err := jobRepo.Upsert(
		context.Background(),
		job.Job{
			Source:      "test",
			ExternalID:  "resume-generator-test",
			Company:     "Setu",
			Title:       "SDE II",
			Description: "Build scalable backend systems using Go.",
			Location:    "Bengaluru",
			URL:         "https://example.com/resume-generator-test",
		},
	); err != nil {
		db.Close()
		t.Fatal(err)
	}

	j, err := jobRepo.GetBySourceExternalID(
		context.Background(),
		"test",
		"resume-generator-test",
	)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	if err := jobRepo.UpdateStatus(
		context.Background(),
		j.ID,
		job.StatusApproved,
	); err != nil {
		db.Close()
		t.Fatal(err)
	}

	resumeDir := t.TempDir()
	masterPath := filepath.Join(resumeDir, "master_resume.txt")

	master := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Worked with Kafka.
- Reduced production log volume by 92%.
- Owned integrations across 18 external issuers.

EDUCATION

B.E. in Information Science
Ramaiah Institute of Technology
`

	if err := os.WriteFile(
		masterPath,
		[]byte(master),
		0644,
	); err != nil {
		db.Close()
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: masterPath,
	})

	builder := NewResumePromptBuilder(source)

	response := `Arkodeep Koley
Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices for backend systems.
- Worked with Kafka in event-driven backend systems.
- Reduced production log volume by 92%.
- Owned integrations across 18 external issuers.

EDUCATION

B.E. in Information Science
Ramaiah Institute of Technology`

	llm := NewMockResumeLLM(response)

	generator := NewLLMResumeGenerator(
		jobRepo,
		builder,
		llm,
	)

	return db, jobRepo, generator
}

func TestLLMResumeGeneratorCreatesResume(t *testing.T) {
	db, jobRepo, generator := setupLLMResumeGeneratorTest(t)
	defer db.Close()

	jobs, err := jobRepo.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}

	outputPath := filepath.Join(
		t.TempDir(),
		"resume",
		"tailored_resume.txt",
	)

	err = generator.GenerateTailoredResume(
		context.Background(),
		jobs[0].ID,
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

	if !strings.Contains(content, "Arkodeep Koley") {
		t.Fatal("generated resume missing candidate name")
	}

	if !strings.Contains(content, "Go microservices") {
		t.Fatal("generated resume missing tailored experience")
	}

	if !strings.Contains(content, "92%") {
		t.Fatal("generated resume missing factual metric")
	}
}

func TestValidateGeneratedResumeRejectsBadOutput(t *testing.T) {
	tests := []struct {
		name   string
		resume string
	}{
		{
			name:   "empty",
			resume: "",
		},
		{
			name:   "too short",
			resume: "short resume",
		},
		{
			name:   "code fence",
			resume: strings.Repeat("resume ", 30) + "```",
		},
		{
			name: "generation commentary",
			resume: "Here is your tailored resume " +
				strings.Repeat("resume ", 30),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateGeneratedResume(tt.resume); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLLMResumeGeneratorHonorsCancelledContext(t *testing.T) {
	db, _, generator := setupLLMResumeGeneratorTest(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := generator.GenerateTailoredResume(
		ctx,
		1,
		filepath.Join(t.TempDir(), "resume.txt"),
	)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestLLMResumeGeneratorRejectsUnsupportedMetric(t *testing.T) {
	db, jobRepo, _ := setupLLMResumeGeneratorTest(t)
	defer db.Close()

	resumeDir := t.TempDir()
	masterPath := filepath.Join(resumeDir, "master_resume.txt")

	master := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Reduced production log volume by 92%.
- Owned integrations across 18 external issuers.
`

	if err := os.WriteFile(
		masterPath,
		[]byte(master),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: masterPath,
	})

	builder := NewResumePromptBuilder(source)

	badResponse := `Arkodeep Koley
Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Reduced production log volume by 97%.
- Owned integrations across 18 external issuers.

EDUCATION

B.E. in Information Science
Ramaiah Institute of Technology`

	llm := NewMockResumeLLM(badResponse)

	generator := NewLLMResumeGenerator(
		jobRepo,
		builder,
		llm,
	)

	outputPath := filepath.Join(
		t.TempDir(),
		"resume",
		"tailored_resume.txt",
	)

	err := generator.GenerateTailoredResume(
		context.Background(),
		1,
		outputPath,
	)

	if err == nil {
		t.Fatal("expected unsupported metric to be rejected")
	}

	if !strings.Contains(err.Error(), "fact validation") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(outputPath); err == nil {
		t.Fatal("resume artifact should not exist after fact validation failure")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected output path error: %v", err)
	}
}

type sequenceResumeLLM struct {
	responses []string
	calls     int
	prompts   []string
}

func (m *sequenceResumeLLM) Generate(
	ctx context.Context,
	prompt string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.calls++
	m.prompts = append(m.prompts, prompt)

	if len(m.responses) == 0 {
		return "", fmt.Errorf("no response configured")
	}

	index := m.calls - 1
	if index >= len(m.responses) {
		index = len(m.responses) - 1
	}

	return m.responses[index], nil
}

func TestLLMResumeGeneratorRetriesAfterFactValidationFailure(t *testing.T) {
	db, jobRepo, _ := setupLLMResumeGeneratorTest(t)
	defer db.Close()

	resumeDir := t.TempDir()
	masterPath := filepath.Join(resumeDir, "master_resume.txt")
	outputPath := filepath.Join(resumeDir, "tailored_resume.txt")

	master := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Reduced production log volume by 92%.
- Owned integrations across 18 external issuers.
`

	if err := os.WriteFile(masterPath, []byte(master), 0644); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: masterPath,
	})

	builder := NewResumePromptBuilder(source)

	invalid := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Reduced production log volume by 97%.
`

	valid := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Reduced production log volume by 92%.
- Owned integrations across 18 external issuers.
`

	llm := &sequenceResumeLLM{
		responses: []string{invalid, valid},
	}

	generator := NewLLMResumeGenerator(
		jobRepo,
		builder,
		llm,
	)

	if err := generator.GenerateTailoredResume(
		context.Background(),
		1,
		outputPath,
	); err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}

	if llm.calls != 2 {
		t.Fatalf("LLM calls = %d, want 2", llm.calls)
	}

	if len(llm.prompts) != 2 {
		t.Fatalf("prompts = %d, want 2", len(llm.prompts))
	}

	if !strings.Contains(
		llm.prompts[1],
		"unsupported metric",
	) {
		t.Fatal("retry prompt does not contain validation feedback")
	}

	generated, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(generated) != strings.TrimSpace(valid) {
		t.Fatalf(
			"generated resume = %q, want valid response",
			string(generated),
		)
	}
}

func TestLLMResumeGeneratorStopsAfterThreeFailedAttempts(t *testing.T) {
	db, jobRepo, _ := setupLLMResumeGeneratorTest(t)
	defer db.Close()

	resumeDir := t.TempDir()
	masterPath := filepath.Join(resumeDir, "master_resume.txt")
	outputPath := filepath.Join(resumeDir, "tailored_resume.txt")

	master := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Reduced production log volume by 92%.
`

	if err := os.WriteFile(masterPath, []byte(master), 0644); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: masterPath,
	})

	builder := NewResumePromptBuilder(source)

	invalid := `Arkodeep Koley

Software Development Engineer

EXPERIENCE

Twid — Software Development Engineer

- Reduced production log volume by 97%.
`

	llm := &sequenceResumeLLM{
		responses: []string{invalid},
	}

	generator := NewLLMResumeGenerator(
		jobRepo,
		builder,
		llm,
	)

	err := generator.GenerateTailoredResume(
		context.Background(),
		1,
		outputPath,
	)

	if err == nil {
		t.Fatal("expected validation failure")
	}

	if !strings.Contains(
		err.Error(),
		"after 3 attempts",
	) {
		t.Fatalf("unexpected error: %v", err)
	}

	if llm.calls != 3 {
		t.Fatalf("LLM calls = %d, want 3", llm.calls)
	}

	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatal("invalid resume should not be written")
	}
}

func TestLLMResumeGeneratorRetryPromptContainsValidationFeedback(t *testing.T) {
	prompt := buildResumeRetryPrompt(
		"ORIGINAL PROMPT",
		fmt.Errorf(`generated resume contains unsupported metric "78%%"`),
	)

	if !strings.Contains(prompt, "ORIGINAL PROMPT") {
		t.Fatal("retry prompt lost original prompt")
	}

	if !strings.Contains(prompt, `unsupported metric "78%`) {
		t.Fatal("retry prompt lost validation error")
	}

	if !strings.Contains(
		prompt,
		"Do NOT replace an unsupported metric with another number",
	) {
		t.Fatal("retry prompt missing metric safety rule")
	}
}
