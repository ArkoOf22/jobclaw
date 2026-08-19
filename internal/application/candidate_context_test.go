package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jobclaw/internal/config"
)

func TestCandidateContextBuilderIncludesCandidateProfileAndResume(t *testing.T) {
	dir := t.TempDir()

	resumePath := filepath.Join(dir, "master_resume.txt")

	err := os.WriteFile(
		resumePath,
		[]byte("MASTER RESUME FACT: Built Go payment systems."),
		0600,
	)
	if err != nil {
		t.Fatal(err)
	}

	resume := NewResumeSource(ResumeSourceConfig{
		MasterPath: resumePath,
	})

	builder := NewCandidateContextBuilder(
		config.Candidate{
			Name: "Arkodeep Koley",
			CurrentRole: config.CurrentRole{
				Title:    "Software Development Engineer",
				Company:  "Twid",
				Location: "Bangalore, India",
				Started:  "2025-01",
			},
			Experience: config.Experience{
				TotalYears:    2,
				PrimaryDomain: "Backend Engineering",
			},
			Skills: config.Skills{
				Languages: []string{"Go", "Java"},
				Backend:   []string{"Kafka", "Redis"},
			},
			DomainExperience: []string{
				"Fintech",
				"Payments",
			},
			Achievements: []string{
				"Winner of Twid Hackathon.",
			},
			Education: config.Education{
				Degree:         "B.E. in Information Science",
				Institution:    "Ramaiah Institute of Technology",
				GraduationYear: 2024,
				CGPA:           9.17,
			},
		},
		resume,
	)

	context, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"Arkodeep Koley",
		"Software Development Engineer",
		"Twid",
		"Bangalore, India",
		"2.0 years",
		"Backend Engineering",
		"- Go",
		"- Kafka",
		"- Fintech",
		"Winner of Twid Hackathon.",
		"B.E. in Information Science",
		"Ramaiah Institute of Technology",
		"9.17",
		"MASTER RESUME FACT: Built Go payment systems.",
	}

	for _, value := range expected {
		if !strings.Contains(context, value) {
			t.Fatalf(
				"context missing %q:\n%s",
				value,
				context,
			)
		}
	}
}

func TestCandidateContextBuilderRequiresCandidateName(t *testing.T) {
	resume := NewResumeSource(ResumeSourceConfig{
		MasterPath: "unused",
	})

	builder := NewCandidateContextBuilder(
		config.Candidate{},
		resume,
	)

	_, err := builder.Build()
	if err == nil {
		t.Fatal("expected candidate name error")
	}
}

func TestCandidateContextBuilderRequiresResume(t *testing.T) {
	builder := NewCandidateContextBuilder(
		config.Candidate{
			Name: "Arkodeep Koley",
		},
		nil,
	)

	_, err := builder.Build()
	if err == nil {
		t.Fatal("expected resume source error")
	}
}
