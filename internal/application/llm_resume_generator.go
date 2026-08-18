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

	const maxAttempts = 3

	currentPrompt := prompt

	var generated string
	var lastValidationErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		generated, err = g.llm.Generate(ctx, currentPrompt)
		if err != nil {
			return fmt.Errorf("generate tailored resume: %w", err)
		}

		generated = strings.TrimSpace(generated)

		if generated == "" {
			lastValidationErr = fmt.Errorf("generated resume is empty")
		} else {
			lastValidationErr = validateGeneratedResumeWithFacts(
				generated,
				g.builder.source,
			)
		}

		if lastValidationErr == nil {
			break
		}

		if attempt == maxAttempts {
			break
		}

		currentPrompt = buildResumeRetryPrompt(
			prompt,
			lastValidationErr,
		)
	}

	if lastValidationErr != nil {
		return fmt.Errorf(
			"resume fact validation failed after %d attempts: %w",
			maxAttempts,
			lastValidationErr,
		)
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

func validateGeneratedResumeWithFacts(
	generated string,
	source *ResumeSource,
) error {
	if source == nil {
		return fmt.Errorf("resume source is required")
	}

	masterResume, err := source.Load()
	if err != nil {
		return fmt.Errorf("load master resume: %w", err)
	}

	guard := NewResumeFactGuard(masterResume)

	if err := guard.Validate(generated); err != nil {
		return err
	}

	return validateGeneratedResume(generated)
}

func buildResumeRetryPrompt(
	originalPrompt string,
	validationErr error,
) string {
	return fmt.Sprintf(`%s

IMPORTANT: The previous generated resume failed factual validation.

Validation failure:
%s

Regenerate the resume from the master resume.

Rules for this retry:
- Remove the unsupported claim completely.
- Do NOT replace an unsupported metric with another number.
- Do NOT invent or estimate a metric.
- Do NOT invent experience, companies, technologies, dates, or achievements.
- Use only facts supported by the master resume.
- Return only the resume.
`,
		originalPrompt,
		validationErr,
	)
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
