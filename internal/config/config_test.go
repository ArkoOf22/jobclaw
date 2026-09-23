package config

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	root := filepath.Join("..", "..")

	cfg, err := Load(
		filepath.Join(root, "config", "candidate.yaml"),
		filepath.Join(root, "config", "preferences.yaml"),
	)

	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Candidate.Candidate.Name != "Arkodeep Koley" {
		t.Fatalf("unexpected candidate name: %q",
			cfg.Candidate.Candidate.Name)
	}

	if !cfg.Preferences.JobPreferences.CompanyType.RequireProductCompany {
		t.Fatal("product company requirement should be enabled")
	}

	if cfg.Preferences.JobPreferences.ApplicationPolicy.AllowAutonomousSubmission {
		t.Fatal("autonomous submission must be disabled")
	}

	if cfg.Resume.Resume.MasterPath != "data/resume/master_resume.txt" {
		t.Fatalf(
			"unexpected resume master path: %q",
			cfg.Resume.Resume.MasterPath,
		)
	}

	if cfg.Resume.Resume.LLM.Provider != "google" {
		t.Fatalf(
			"unexpected resume LLM provider: %q",
			cfg.Resume.Resume.LLM.Provider,
		)
	}

	if cfg.Resume.Resume.LLM.APIKeyEnv != "GEMINI_API_KEY" {
		t.Fatalf(
			"unexpected API key env: %q",
			cfg.Resume.Resume.LLM.APIKeyEnv,
		)
	}

	if cfg.Resume.Resume.Generation.AllowMetricChanges {
		t.Fatal("resume metric changes must remain disabled")
	}

	if cfg.Resume.Resume.Generation.AllowExperienceInvention {
		t.Fatal("resume experience invention must remain disabled")
	}
}
