package application

import (
	"fmt"
	"strings"

	"jobclaw/internal/config"
)

type CandidateContextBuilder struct {
	candidate config.Candidate
	resume    *ResumeSource
}

func NewCandidateContextBuilder(
	candidate config.Candidate,
	resume *ResumeSource,
) *CandidateContextBuilder {
	return &CandidateContextBuilder{
		candidate: candidate,
		resume:    resume,
	}
}

func (b *CandidateContextBuilder) Build() (string, error) {
	if strings.TrimSpace(b.candidate.Name) == "" {
		return "", fmt.Errorf("candidate name is required")
	}

	if b.resume == nil {
		return "", fmt.Errorf("resume source is required")
	}

	masterResume, err := b.resume.Load()
	if err != nil {
		return "", fmt.Errorf("load master resume: %w", err)
	}

	var out strings.Builder

	out.WriteString("CANDIDATE PROFILE\n")
	out.WriteString("=================\n\n")

	fmt.Fprintf(&out, "Name: %s\n", b.candidate.Name)

	if b.candidate.CurrentRole.Title != "" {
		fmt.Fprintf(
			&out,
			"Current Role: %s at %s\n",
			b.candidate.CurrentRole.Title,
			b.candidate.CurrentRole.Company,
		)
	}

	if b.candidate.CurrentRole.Location != "" {
		fmt.Fprintf(
			&out,
			"Location: %s\n",
			b.candidate.CurrentRole.Location,
		)
	}

	if b.candidate.CurrentRole.Started != "" {
		fmt.Fprintf(
			&out,
			"Started Current Role: %s\n",
			b.candidate.CurrentRole.Started,
		)
	}

	if b.candidate.Experience.TotalYears > 0 {
		fmt.Fprintf(
			&out,
			"Total Experience: %.1f years\n",
			b.candidate.Experience.TotalYears,
		)
	}

	if b.candidate.Experience.PrimaryDomain != "" {
		fmt.Fprintf(
			&out,
			"Primary Domain: %s\n",
			b.candidate.Experience.PrimaryDomain,
		)
	}

	writeList(&out, "Primary Target Roles", b.candidate.TargetRoles.Primary)
	writeList(&out, "Secondary Target Roles", b.candidate.TargetRoles.Secondary)

	writeList(&out, "Languages", b.candidate.Skills.Languages)
	writeList(&out, "Backend Skills", b.candidate.Skills.Backend)
	writeList(&out, "Databases", b.candidate.Skills.Databases)
	writeList(
		&out,
		"Cloud / Infrastructure",
		b.candidate.Skills.CloudInfrastructure,
	)
	writeList(&out, "Observability", b.candidate.Skills.Observability)
	writeList(&out, "AI Engineering", b.candidate.Skills.AIEngineering)

	writeList(&out, "Domain Experience", b.candidate.DomainExperience)
	writeList(
		&out,
		"Experience Highlights",
		b.candidate.ExperienceHighlights,
	)
	writeList(&out, "Achievements", b.candidate.Achievements)

	if b.candidate.Education.Degree != "" {
		out.WriteString("\nEDUCATION\n")
		out.WriteString("---------\n")
		fmt.Fprintf(
			&out,
			"Degree: %s\n",
			b.candidate.Education.Degree,
		)
		fmt.Fprintf(
			&out,
			"Institution: %s\n",
			b.candidate.Education.Institution,
		)

		if b.candidate.Education.GraduationYear > 0 {
			fmt.Fprintf(
				&out,
				"Graduation Year: %d\n",
				b.candidate.Education.GraduationYear,
			)
		}

		if b.candidate.Education.CGPA > 0 {
			fmt.Fprintf(
				&out,
				"CGPA: %.2f\n",
				b.candidate.Education.CGPA,
			)
		}
	}

	out.WriteString("\nMASTER RESUME\n")
	out.WriteString("=============\n\n")
	out.WriteString(strings.TrimSpace(masterResume))
	out.WriteString("\n")

	return out.String(), nil
}

func writeList(
	out *strings.Builder,
	title string,
	values []string,
) {
	if len(values) == 0 {
		return
	}

	fmt.Fprintf(out, "\n%s:\n", title)

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		fmt.Fprintf(out, "- %s\n", value)
	}
}
