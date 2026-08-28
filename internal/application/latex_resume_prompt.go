package application

import (
	"fmt"
	"strings"

	"jobclaw/internal/job"
)

// buildLaTeXResumePrompt asks the model to return the tailored resume as
// structured JSON rather than prose, so it can be rendered into a fixed LaTeX
// template deterministically. The factual-integrity rules mirror the plain-text
// generator: the master resume is the only source of truth, and nothing may be
// invented from the job description.
func buildLaTeXResumePrompt(masterResume string, j *job.Job) (string, error) {
	if j == nil {
		return "", fmt.Errorf("job is required")
	}

	if strings.TrimSpace(masterResume) == "" {
		return "", fmt.Errorf("master resume is required")
	}

	description := capJobDescription(
		NormalizeJobDescription(j.Description),
		maxJobDescriptionChars,
	)

	var p strings.Builder

	p.WriteString(`You are JobClaw's resume tailoring engine. Produce a resume tailored to the target job, drawn strictly from the candidate's master resume.

Return ONLY a single JSON object, no prose, no markdown fences. The object has exactly these keys:

{
  "skills": [ { "label": "Languages", "value": "Go, Java, SQL, Python" } ],
  "achievement_line": "Solved 300+ Data Structures and Algorithms problems on LeetCode.",
  "experience": [
    {
      "company": "Twid",
      "role": "Software Development Engineer",
      "tech": "Go, Kafka, PostgreSQL, Redis",
      "dates": "January 2025 -- Present",
      "bullets": [ "Owned integrations across 18 external banking and loyalty issuers." ]
    }
  ],
  "projects": [
    {
      "name": "Distributed Service Orchestrator",
      "tech": "Go, Kafka, gRPC, Redis, AWS",
      "github": "",
      "bullets": [ "Built a distributed orchestration system coordinating 4+ internal services." ]
    }
  ]
}

FACTUAL INTEGRITY (these are absolute):
1. The MASTER RESUME is the only source of truth. The JOB DESCRIPTION is a relevance signal only; it is never evidence that the candidate has a skill or did a thing.
2. Do not invent companies, roles, dates, technologies, responsibilities, metrics, or achievements.
3. Every number must appear verbatim in the master resume. Never invent, estimate, round, or combine numbers into a new figure.
4. If the job wants something the master resume does not support, omit it. Do not fabricate.

TAILORING (allowed):
5. Select and reorder the most relevant existing experience, projects, skills, and bullets for this job.
6. Reword bullets for clarity and relevance without changing their factual content.
7. Choose which skill lines and bullets to include; you may drop less relevant ones to keep the resume focused.

FORMAT RULES:
8. Values must be PLAIN TEXT. Do not include any LaTeX commands, backslashes, or special formatting. Escaping is handled downstream.
9. "tech" is a short comma-separated list of technologies for that role or project, drawn from the master resume. Omit the surrounding parentheses.
10. Preserve the candidate's real employment history and dates exactly as in the master resume.
11. Keep every company and project that appears in the master resume; tailoring selects bullets, it does not drop whole jobs.

TARGET JOB

Company:
`)
	p.WriteString(strings.TrimSpace(j.Company))
	p.WriteString("\n\nTitle:\n")
	p.WriteString(strings.TrimSpace(j.Title))
	p.WriteString("\n\nJob Description:\n")
	p.WriteString(description)
	p.WriteString("\n\nMASTER RESUME\n\n")
	p.WriteString(masterResume)
	p.WriteString("\n\nReturn only the JSON object described above.")

	return p.String(), nil
}
