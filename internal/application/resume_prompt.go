package application

import (
	"fmt"
	"strings"

	"jobclaw/internal/job"
)

// maxJobDescriptionChars caps the description included in the prompt.
//
// Requirements and responsibilities sit well inside this on real postings; what
// exceeds it is employer marketing. Roughly 1500 tokens, against a master resume
// of about 700.
const maxJobDescriptionChars = 6000

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

SOURCE AUTHORITY:
1. The MASTER RESUME is the only factual source of truth.
2. The TARGET JOB DESCRIPTION is only a relevance signal. It must NEVER be treated as evidence that the candidate has a skill, technology, responsibility, achievement, or experience.
3. If a fact is not supported by the MASTER RESUME, omit it.

EXPERIENCE AND TECHNOLOGY:
4. Do not invent experience.
5. Do not invent companies or institutions.
6. Do not invent technologies, tools, frameworks, languages, databases, or infrastructure.
7. Do not invent responsibilities or ownership.
8. Do not invent dates or employment history.
9. Do not invent achievements or certifications.
10. Do not claim the candidate used a technology merely because it appears in the job description.

METRICS AND NUMBERS:
11. Every numerical value in the tailored resume must be supported by the MASTER RESUME.
12. Preserve numerical values exactly as written in the MASTER RESUME.
13. Never invent, estimate, infer, calculate, derive, round, or extrapolate a metric.
14. Never create a percentage that does not explicitly exist in the MASTER RESUME.
15. Never combine multiple facts to create a new numerical claim.
16. Never convert units or formats, such as 15K into 15,000, unless the exact form already exists in the MASTER RESUME.
17. If you are uncertain whether a number or metric is supported, omit it.

ALLOWED TRANSFORMATIONS:
18. You may rewrite wording to improve relevance and clarity.
19. You may reorder sections and bullets.
20. You may select the most relevant existing skills, projects, and experience.
21. You may combine existing facts into a clearer sentence, but the resulting sentence must not introduce any new factual claim.
22. Preserve the candidate's actual employment history.

SAFETY RULE:
23. If the job description asks for something that is not supported by the MASTER RESUME, do not fabricate evidence for it. Omit the unsupported claim instead.

FINAL CHECK:
24. Before returning the resume, verify that every metric, technology, company, date, and achievement is supported by the MASTER RESUME.
25. If a statement cannot be traced back to the MASTER RESUME, remove it.

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

	// Descriptions are stored as escaped HTML. Sending them raw bills the model
	// to read markup, so normalise to plain text and drop employer boilerplate
	// before including it. The description is only a relevance signal, per the
	// rules above, so nothing factual is lost.
	description := TrimJobDescription(
		NormalizeJobDescription(j.Description),
		maxJobDescriptionChars,
	)

	prompt.WriteString("Job Description:\n")
	prompt.WriteString(description)
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

The output must remain completely grounded in the master resume.

FINAL INSTRUCTION:
When choosing between a stronger-sounding statement and a strictly supported statement, always choose the strictly supported statement.`)

	return prompt.String(), nil
}
