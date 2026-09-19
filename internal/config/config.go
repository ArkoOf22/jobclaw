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
	ExperienceRequired    ExperienceRequired    `yaml:"experience"`
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

	// Excluded titles are a hard veto: a job whose title contains any of these
	// is forced to SKIP regardless of score. Reserve this for genuinely
	// unreachable roles (Director, VP, Principal, Staff, Manager) and clear
	// role mismatches (Frontend, QA, Data Analyst).
	Excluded []string `yaml:"excluded"`

	// Demoted titles are NOT vetoed. They stay visible but take a ranking
	// penalty, so a borderline-senior role the candidate could legitimately
	// apply to (in India "Senior"/"Sr"/"Lead" often means 3-4 years) still
	// surfaces instead of vanishing. The penalty is applied in the scorer.
	Demoted []string `yaml:"demoted"`
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

// ExperienceRequired bounds how much experience a posting may demand.
//
// This is a hard filter rather than a scoring weight. Years demanded is the one
// requirement a candidate cannot narrow the gap on by being a strong match
// elsewhere, and treating it as a score let "8+ years" roles reach the shortlist
// on the strength of their stack.
type ExperienceRequired struct {
	// MaxRequiredYears is the comfortable target: postings at or below it score
	// full marks on experience. It is no longer the veto line — the graded
	// experience score and this value together let a slightly-over role rank
	// lower rather than disappear.
	MaxRequiredYears float64 `yaml:"max_required_years"`

	// HardCeilingYears is the genuine reach limit: a posting stating more than
	// this is vetoed to SKIP, because it is a different candidate's job, not a
	// stretch. Zero falls back to a sensible default (defaultExperienceHardCeiling
	// in the scorer) so the veto is never accidentally disabled, but it stays
	// well above MaxRequiredYears so "3-5 years" roles remain visible.
	HardCeilingYears float64 `yaml:"hard_ceiling_years"`
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
