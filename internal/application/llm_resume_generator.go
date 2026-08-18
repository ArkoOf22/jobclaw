package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jobclaw/internal/job"
)

type LLMResumeGenerator struct {
	jobs    *job.SQLiteRepository
	builder *ResumePromptBuilder
	llm     ResumeLLM
}

func NewLLMResumeGenerator(
	jobs *job.SQLiteRepository,
	builder *ResumePromptBuilder,
	llm ResumeLLM,
) *LLMResumeGenerator {
	return &LLMResumeGenerator{
		jobs:    jobs,
		builder: builder,
		llm:     llm,
	}
}

func (g *LLMResumeGenerator) GenerateTailoredResume(
	ctx context.Context,
	jobID int64,
	outputPath string,
) error {
	if jobID <= 0 {
		return fmt.Errorf("job ID must be positive")
	}

	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("output path is required")
	}

	if g.jobs == nil {
		return fmt.Errorf("job repository is required")
	}

	if g.builder == nil {
		return fmt.Errorf("resume prompt builder is required")
	}

	if g.llm == nil {
		return fmt.Errorf("resume LLM is required")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	j, err := g.jobs.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}

	if j == nil {
		return fmt.Errorf("job %d not found", jobID)
	}

	prompt, err := g.builder.Build(j)
	if err != nil {
		return fmt.Errorf("build resume prompt: %w", err)
	}

	generated, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate tailored resume: %w", err)
	}

	generated = strings.TrimSpace(generated)

	if generated == "" {
		return fmt.Errorf("generated resume is empty")
	}

	if err := validateGeneratedResume(generated); err != nil {
		return fmt.Errorf("validate generated resume: %w", err)
	}

	masterResume, err := g.builder.source.Load()
	if err != nil {
		return fmt.Errorf("load master resume for fact validation: %w", err)
	}

	factGuard := NewResumeFactGuard(masterResume)

	if err := factGuard.Validate(generated); err != nil {
		return fmt.Errorf("resume fact validation failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create resume directory: %w", err)
	}

	if err := os.WriteFile(
		outputPath,
		[]byte(generated),
		0644,
	); err != nil {
		return fmt.Errorf("write tailored resume: %w", err)
	}

	return nil
}

func validateGeneratedResume(resume string) error {
	resume = strings.TrimSpace(resume)

	if resume == "" {
		return fmt.Errorf("resume is empty")
	}

	if len(resume) < 100 {
		return fmt.Errorf("resume output is suspiciously short")
	}

	lower := strings.ToLower(resume)

	if strings.Contains(lower, "```") {
		return fmt.Errorf("resume contains markdown code fences")
	}

	if strings.Contains(lower, "here is your tailored resume") {
		return fmt.Errorf("resume contains generation commentary")
	}

	return nil
}
