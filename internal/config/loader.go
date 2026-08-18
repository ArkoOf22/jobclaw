package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Candidate   CandidateConfig
	Preferences PreferencesConfig
	Resume      ResumeConfig
}

func Load(candidatePath, preferencesPath string) (*Config, error) {
	candidateData, err := os.ReadFile(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("read candidate config: %w", err)
	}

	preferenceData, err := os.ReadFile(preferencesPath)
	if err != nil {
		return nil, fmt.Errorf("read preferences config: %w", err)
	}

	resumePath := filepath.Join(
		filepath.Dir(candidatePath),
		"resume.yaml",
	)

	resumeData, err := os.ReadFile(resumePath)
	if err != nil {
		return nil, fmt.Errorf("read resume config: %w", err)
	}

	var candidate CandidateConfig
	if err := yaml.Unmarshal(candidateData, &candidate); err != nil {
		return nil, fmt.Errorf("parse candidate config: %w", err)
	}

	var preferences PreferencesConfig
	if err := yaml.Unmarshal(preferenceData, &preferences); err != nil {
		return nil, fmt.Errorf("parse preferences config: %w", err)
	}

	var resume ResumeConfig
	if err := yaml.Unmarshal(resumeData, &resume); err != nil {
		return nil, fmt.Errorf("parse resume config: %w", err)
	}

	config := &Config{
		Candidate:   candidate,
		Preferences: preferences,
		Resume:      resume,
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}
