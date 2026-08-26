package config

type ResumeConfig struct {
	Resume Resume `yaml:"resume"`
}

type Resume struct {
	MasterPath   string           `yaml:"master_path"`
	OutputFormat string           `yaml:"output_format"`
	Generation   ResumeGeneration `yaml:"generation"`
	LLM          ResumeLLMConfig  `yaml:"llm"`
}

type ResumeGeneration struct {
	PreserveFacts            bool `yaml:"preserve_facts"`
	AllowRewording           bool `yaml:"allow_rewording"`
	AllowReordering          bool `yaml:"allow_reordering"`
	AllowSkillSelection      bool `yaml:"allow_skill_selection"`
	AllowMetricChanges       bool `yaml:"allow_metric_changes"`
	AllowExperienceInvention bool `yaml:"allow_experience_invention"`
}

type ResumeLLMConfig struct {
	Provider  string           `yaml:"provider"`
	Model     string           `yaml:"model"`
	BaseURL   string           `yaml:"base_url"`
	APIKeyEnv string           `yaml:"api_key_env"`
	Privacy   ResumeLLMPrivacy `yaml:"privacy"`
}

// ResumeLLMPrivacy controls how much the upstream provider is permitted to do
// with prompt content. Resume prompts embed the candidate's full employment
// history, education, and contact details, so these are deliberately
// restrictive by default.
type ResumeLLMPrivacy struct {
	// DenyDataCollection maps to OpenRouter's provider.data_collection="deny".
	// When true, only providers that do not store prompt data may serve the
	// request. Requests fail rather than silently falling back to a provider
	// that trains on inputs.
	DenyDataCollection bool `yaml:"deny_data_collection"`

	// RequireZeroDataRetention maps to OpenRouter's provider.zdr=true,
	// restricting routing to endpoints with a Zero Data Retention policy.
	RequireZeroDataRetention bool `yaml:"require_zero_data_retention"`
}
