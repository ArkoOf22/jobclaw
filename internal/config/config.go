package config

type CandidateConfig struct {
	Candidate Candidate `yaml:"candidate"`
}

type Candidate struct {
	Name                 string      `yaml:"name"`
	Contact              Contact     `yaml:"contact"`
	CurrentRole          CurrentRole `yaml:"current_role"`
	Experience           Experience  `yaml:"experience"`
	TargetRoles          TargetRoles `yaml:"target_roles"`
	Skills               Skills      `yaml:"skills"`
	DomainExperience     []string    `yaml:"domain_experience"`
	ExperienceHighlights []string    `yaml:"experience_highlights"`
	Achievements         []string    `yaml:"achievements"`
	Education            Education   `yaml:"education"`
}

// Contact holds the header details a rendered resume needs. These are identity,
// not tailoring inputs, so they come from config and the model never touches
// them. Empty fields are simply omitted from the header.
type Contact struct {
	Email    string `yaml:"email"`
	Phone    string `yaml:"phone"`
	LinkedIn string `yaml:"linkedin"`
	GitHub   string `yaml:"github"`
	LeetCode string `yaml:"leetcode"`
}

type CurrentRole struct {
	Title    string `yaml:"title"`
	Company  string `yaml:"company"`
	Location string `yaml:"location"`
	Started  string `yaml:"started"`
}

type Experience struct {
	TotalYears    float64 `yaml:"total_years"`
	PrimaryDomain string  `yaml:"primary_domain"`
}

type TargetRoles struct {
	Primary   []string `yaml:"primary"`
	Secondary []string `yaml:"secondary"`
}

type Skills struct {
	Languages           []string `yaml:"languages"`
	Backend             []string `yaml:"backend"`
	Databases           []string `yaml:"databases"`
	CloudInfrastructure []string `yaml:"cloud_infrastructure"`
	Observability       []string `yaml:"observability"`
	AIEngineering       []string `yaml:"ai_engineering"`
}

type Education struct {
	Degree         string  `yaml:"degree"`
	Institution    string  `yaml:"institution"`
	GraduationYear int     `yaml:"graduation_year"`
	CGPA           float64 `yaml:"cgpa"`
}

type PreferencesConfig struct {
	JobPreferences JobPreferences `yaml:"job_preferences"`
}

type JobPreferences struct {
	CompanyType           CompanyType           `yaml:"company_type"`
	Roles                 Roles                 `yaml:"roles"`
	Locations             Locations             `yaml:"locations"`
	WorkMode              WorkMode              `yaml:"work_mode"`
	Compensation          Compensation          `yaml:"compensation"`
	Domains               Domains               `yaml:"domains"`
	TechnologyPreferences TechnologyPreferences `yaml:"technology_preferences"`
	JobQuality            JobQuality            `yaml:"job_quality"`
	ApplicationPolicy     ApplicationPolicy     `yaml:"application_policy"`
	Exclusions            Exclusions            `yaml:"exclusions"`
}

type CompanyType struct {
	Primary               string   `yaml:"primary"`
	Preferred             []string `yaml:"preferred"`
	Excluded              []string `yaml:"excluded"`
	RequireProductCompany bool     `yaml:"require_product_company"`
}

type Roles struct {
	Preferred  []string `yaml:"preferred"`
	Acceptable []string `yaml:"acceptable"`
	Excluded   []string `yaml:"excluded"`
}

type Locations struct {
	Preferred  []string `yaml:"preferred"`
	Acceptable []string `yaml:"acceptable"`
}

type WorkMode struct {
	Preferred []string `yaml:"preferred"`
}

type Compensation struct {
	Currency     string  `yaml:"currency"`
	TargetMinLPA float64 `yaml:"target_min_lpa"`
	TargetMaxLPA float64 `yaml:"target_max_lpa"`
}

type Domains struct {
	StronglyPreferred []string `yaml:"strongly_preferred"`
	Preferred         []string `yaml:"preferred"`
}

type TechnologyPreferences struct {
	StronglyPreferred []string `yaml:"strongly_preferred"`
	Preferred         []string `yaml:"preferred"`
}

type JobQuality struct {
	MinimumScoreForShortlist float64 `yaml:"minimum_score_for_shortlist"`
	MinimumScoreForApply     float64 `yaml:"minimum_score_for_apply"`
	MinimumScoreForAutopilot float64 `yaml:"minimum_score_for_autopilot"`
}

type ApplicationPolicy struct {
	MaxApplicationsPerDay     int  `yaml:"max_applications_per_day"`
	RequireHumanApproval      bool `yaml:"require_human_approval"`
	AllowAutonomousSubmission bool `yaml:"allow_autonomous_submission"`
}

type Exclusions struct {
	Internships                     bool `yaml:"internships"`
	UnpaidRoles                     bool `yaml:"unpaid_roles"`
	RolesWithoutClearJobDescription bool `yaml:"roles_without_clear_job_description"`
	ServiceBasedCompanies           bool `yaml:"service_based_companies"`
}
