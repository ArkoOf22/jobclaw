package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jobclaw/internal/job"
)

// LaTeXResumeGenerator produces a tailored resume as a compiled PDF.
//
// It reuses the plain-text generator's guarantees: the model returns content
// drawn only from the master resume, that content is fact-guarded, and only then
// is it rendered into the fixed template and compiled. Contact and education are
// supplied from config and never pass through the model.
type LaTeXResumeGenerator struct {
	jobs      *job.SQLiteRepository
	source    *ResumeSource
	llm       ResumeLLM
	compiler  PDFCompiler
	contact   ResumeContact
	education []ResumeEducation
}

func NewLaTeXResumeGenerator(
	jobs *job.SQLiteRepository,
	source *ResumeSource,
	llm ResumeLLM,
	compiler PDFCompiler,
	contact ResumeContact,
	education []ResumeEducation,
) *LaTeXResumeGenerator {
	return &LaTeXResumeGenerator{
		jobs:      jobs,
		source:    source,
		llm:       llm,
		compiler:  compiler,
		contact:   contact,
		education: education,
	}
}

// ResumeArtifacts are the files a generation run produces.
type ResumeArtifacts struct {
	PDFPath  string
	TextPath string
}

// GenerateResumePDF tailors a resume for a job and produces both a compiled PDF
// and a plain-text rendering in workDir. A single model call feeds both, so the
// text artifact the readiness check needs costs nothing extra.
func (g *LaTeXResumeGenerator) GenerateResumePDF(
	ctx context.Context,
	jobID int64,
	workDir string,
) (ResumeArtifacts, error) {
	if g.jobs == nil || g.source == nil || g.llm == nil || g.compiler == nil {
		return ResumeArtifacts{}, fmt.Errorf(
			"latex resume generator is not fully configured",
		)
	}

	j, err := g.jobs.GetByID(ctx, jobID)
	if err != nil {
		return ResumeArtifacts{}, fmt.Errorf("load job: %w", err)
	}

	if j == nil {
		return ResumeArtifacts{}, fmt.Errorf("job %d not found", jobID)
	}

	master, err := g.source.Load()
	if err != nil {
		return ResumeArtifacts{}, fmt.Errorf("load master resume: %w", err)
	}

	prompt, err := buildLaTeXResumePrompt(master, j)
	if err != nil {
		return ResumeArtifacts{}, fmt.Errorf("build resume prompt: %w", err)
	}

	content, err := g.generateContent(ctx, prompt, master)
	if err != nil {
		return ResumeArtifacts{}, err
	}

	latex := RenderResumeLaTeX(g.contact, g.education, content)

	pdfPath, err := g.compiler.Compile(
		ctx,
		latex,
		workDir,
		"tailored_resume.pdf",
	)
	if err != nil {
		return ResumeArtifacts{}, fmt.Errorf("compile resume: %w", err)
	}

	textPath := filepath.Join(workDir, "tailored_resume.txt")

	plainText := RenderResumePlainText(g.contact, g.education, content)

	if err := os.WriteFile(textPath, []byte(plainText), 0644); err != nil {
		return ResumeArtifacts{}, fmt.Errorf("write text resume: %w", err)
	}

	return ResumeArtifacts{PDFPath: pdfPath, TextPath: textPath}, nil
}

// generateContent calls the model, parses its JSON, and fact-guards the result,
// retrying on a parse or validation failure with a corrective instruction.
func (g *LaTeXResumeGenerator) generateContent(
	ctx context.Context,
	prompt string,
	master string,
) (ResumeContent, error) {
	const maxAttempts = 3

	currentPrompt := prompt
	guard := NewResumeFactGuard(master)

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return ResumeContent{}, err
		}

		raw, err := g.llm.Generate(ctx, currentPrompt)
		if err != nil {
			return ResumeContent{}, fmt.Errorf("generate resume content: %w", err)
		}

		content, err := parseResumeContent(raw)
		if err != nil {
			lastErr = err
		} else if err := guard.Validate(content.textFields()); err != nil {
			lastErr = fmt.Errorf("fact validation failed: %w", err)
		} else if len(content.Experience) == 0 {
			lastErr = fmt.Errorf("no experience entries produced")
		} else {
			return content, nil
		}

		if attempt < maxAttempts {
			currentPrompt = fmt.Sprintf(
				"%s\n\nThe previous response was rejected: %s\nReturn only the corrected JSON object, drawn strictly from the master resume.",
				prompt,
				lastErr,
			)
		}
	}

	return ResumeContent{}, fmt.Errorf(
		"resume content generation failed after %d attempts: %w",
		maxAttempts,
		lastErr,
	)
}

// parseResumeContent decodes the model's reply into ResumeContent, tolerating a
// reply wrapped in markdown fences or padded with surrounding prose by
// extracting the outermost JSON object.
func parseResumeContent(raw string) (ResumeContent, error) {
	cleaned := extractJSONObject(raw)
	if cleaned == "" {
		return ResumeContent{}, fmt.Errorf("no JSON object found in model reply")
	}

	var content ResumeContent

	if err := json.Unmarshal([]byte(cleaned), &content); err != nil {
		return ResumeContent{}, fmt.Errorf("decode resume JSON: %w", err)
	}

	return content, nil
}

// extractJSONObject returns the substring from the first "{" to the last "}",
// after stripping any markdown code fences. The model is told to return bare
// JSON, but this makes the parse robust to the common deviations.
func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")

	if start < 0 || end < 0 || end < start {
		return ""
	}

	return raw[start : end+1]
}
