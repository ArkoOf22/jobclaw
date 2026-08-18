package config

import (
	"fmt"
	"strings"
)

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Candidate.Candidate.Name) == "" {
		return fmt.Errorf("candidate name is required")
	}

	if strings.TrimSpace(c.Candidate.Candidate.CurrentRole.Title) == "" {
		return fmt.Errorf("candidate current role title is required")
	}

	if strings.TrimSpace(c.Candidate.Candidate.CurrentRole.Company) == "" {
		return fmt.Errorf("candidate current company is required")
	}

	if len(c.Candidate.Candidate.TargetRoles.Primary) == 0 {
		return fmt.Errorf("at least one primary target role is required")
	}

	if c.Preferences.JobPreferences.CompanyType.RequireProductCompany &&
		len(c.Preferences.JobPreferences.CompanyType.Preferred) == 0 {
		return fmt.Errorf("product-company preference requires preferred company types")
	}

	compensation := c.Preferences.JobPreferences.Compensation

	if compensation.TargetMinLPA < 0 ||
		compensation.TargetMaxLPA < compensation.TargetMinLPA {
		return fmt.Errorf("invalid compensation range")
	}

	quality := c.Preferences.JobPreferences.JobQuality

	if quality.MinimumScoreForShortlist < 0 ||
		quality.MinimumScoreForApply < quality.MinimumScoreForShortlist ||
		quality.MinimumScoreForAutopilot < quality.MinimumScoreForApply {
		return fmt.Errorf("invalid job score thresholds")
	}

	policy := c.Preferences.JobPreferences.ApplicationPolicy

	if policy.MaxApplicationsPerDay <= 0 {
		return fmt.Errorf("max applications per day must be positive")
	}

	if policy.AllowAutonomousSubmission {
		return fmt.Errorf("autonomous submission must remain disabled during foundation")
	}

	resume := c.Resume.Resume

	if strings.TrimSpace(resume.MasterPath) == "" {
		return fmt.Errorf("resume master path is required")
	}

	if strings.TrimSpace(resume.OutputFormat) == "" {
		return fmt.Errorf("resume output format is required")
	}

	if strings.TrimSpace(resume.LLM.Provider) == "" {
		return fmt.Errorf("resume LLM provider is required")
	}

	if strings.TrimSpace(resume.LLM.Model) == "" {
		return fmt.Errorf("resume LLM model is required")
	}

	if strings.TrimSpace(resume.LLM.APIKeyEnv) == "" {
		return fmt.Errorf("resume LLM API key environment variable is required")
	}

	if resume.Generation.AllowMetricChanges {
		return fmt.Errorf("resume metric changes must remain disabled")
	}

	if resume.Generation.AllowExperienceInvention {
		return fmt.Errorf("resume experience invention must remain disabled")
	}

	return nil
}
