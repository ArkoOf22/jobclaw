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
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
}
