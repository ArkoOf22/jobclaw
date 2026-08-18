package application

import (
	"fmt"
	"os"
	"path/filepath"
)

type ResumeSourceConfig struct {
	MasterPath   string
	OutputFormat string

	PreserveFacts         bool
	AllowRewording        bool
	AllowReordering       bool
	AllowSkillSelection   bool
	AllowMetricChanges    bool
	AllowExperienceInvent bool
}

type ResumeSource struct {
	config ResumeSourceConfig
}

func NewResumeSource(config ResumeSourceConfig) *ResumeSource {
	return &ResumeSource{
		config: config,
	}
}

func (r *ResumeSource) Load() (string, error) {
	path := filepath.Clean(r.config.MasterPath)

	if path == "." || path == "" {
		return "", fmt.Errorf("master resume path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read master resume: %w", err)
	}

	content := string(data)
	if len(content) == 0 {
		return "", fmt.Errorf("master resume is empty")
	}

	return content, nil
}
