package application

import (
	"fmt"
	"strings"

	"jobclaw/internal/job"
)

type ResumePromptBuilder struct {
	source *ResumeSource
}

func NewResumePromptBuilder(source *ResumeSource) *ResumePromptBuilder {
	return &ResumePromptBuilder{
		source: source,
	}
}

func (b *ResumePromptBuilder) Build(j *job.Job) (string, error) {
	if j == nil {
		return "", fmt.Errorf("job is required")
	}

	if j.ID <= 0 {
		return "", fmt.Errorf("job ID must be positive")
	}

	if strings.TrimSpace(j.Title) == "" {
		return "", fmt.Errorf("job title is required")
	}

	if strings.TrimSpace(j.Description) == "" {
		return "", fmt.Errorf("job description is required")
	}

	if b.source == nil {
		return "", fmt.Errorf("resume source is required")
	}

	masterResume, err := b.source.Load()
	if err != nil {
		return "", fmt.Errorf("load master resume: %w", err)
	}

	var prompt strings.Builder

	prompt.WriteString(`You are JobClaw's resume tailoring engine.

Your task is to tailor the candidate's master resume for the target job.

IMPORTANT FACTUAL INTEGRITY RULES:
1. The master resume is the factual source of truth.
2. Do not invent experience.
3. Do not invent companies.
4. Do not invent technologies.
5. Do not invent dates.
6. Do not invent achievements.
7. Do not invent metrics.
8. Do not change numerical values from the master resume.
9. Do not claim the candidate used a technology unless it appears in the master resume.
10. You may rewrite wording to improve relevance and clarity.
11. You may reorder sections and bullets.
12. You may select the most relevant existing skills and experience.
13. Preserve the candidate's actual employment history.
14. If the job description asks for something not supported by the master resume, do not fabricate evidence for it.

TARGET JOB

Job ID:
`)
	fmt.Fprintf(&prompt, "%d\n\n", j.ID)

	prompt.WriteString("Company:\n")
	prompt.WriteString(strings.TrimSpace(j.Company))
	prompt.WriteString("\n\n")

	prompt.WriteString("Title:\n")
	prompt.WriteString(strings.TrimSpace(j.Title))
	prompt.WriteString("\n\n")

	prompt.WriteString("Location:\n")
	prompt.WriteString(strings.TrimSpace(j.Location))
	prompt.WriteString("\n\n")

	prompt.WriteString("Job Description:\n")
	prompt.WriteString(strings.TrimSpace(j.Description))
	prompt.WriteString("\n\n")

	prompt.WriteString("MASTER RESUME\n\n")
	prompt.WriteString(masterResume)
	prompt.WriteString("\n\n")

	prompt.WriteString(`OUTPUT REQUIREMENTS

Return only the tailored resume.

Do not include:
- explanations
- analysis
- commentary
- markdown fences
- claims about the tailoring process

The output must remain completely grounded in the master resume.`)

	return prompt.String(), nil
}
