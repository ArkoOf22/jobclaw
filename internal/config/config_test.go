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
}
