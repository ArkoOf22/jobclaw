package scoring

import (
	"fmt"
	"strings"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
	"jobclaw/internal/job"
)

type Recommendation string

const (
	RecommendationSkip      Recommendation = "SKIP"
	RecommendationShortlist Recommendation = "SHORTLIST"
	RecommendationApply     Recommendation = "APPLY"
)

type Result struct {
	OverallScore         float64
	SkillsScore          float64
	CandidateSkillsScore float64
	RoleScore            float64
	ExperienceScore      float64
	DomainScore          float64
	CandidateDomainScore float64
	LocationScore        float64
	CompanyScore         float64
	CompensationScore    float64

	Recommendation Recommendation
	Reasoning      string
}

type Scorer struct {
	candidate   config.Candidate
	preferences config.JobPreferences
	matcher     *CandidateMatcher

	// titleFamily is the set of related job titles the candidate is targeting,
	// seeded from candidate.yaml target_roles (primary + secondary). It lets
	// scoreRole credit sibling titles the way LinkedIn/Naukri do, and it is
	// personalised and editable in one place the candidate already owns.
	titleFamily []string
}

func NewScorer(
	candidate config.Candidate,
	preferences config.JobPreferences,
) *Scorer {
	return &Scorer{
		candidate:   candidate,
		preferences: preferences,
		matcher:     NewCandidateMatcher(candidate),
		titleFamily: buildTitleFamily(candidate.TargetRoles),
	}
}

// buildTitleFamily flattens the candidate's primary and secondary target roles
// into a single de-duplicated list. These are the titles the candidate already
// declared they want, so treating them as a family adds no new assumptions — it
// just stops the scorer from demanding an exact match against the preferences
// role lists.
func buildTitleFamily(roles config.TargetRoles) []string {
	combined := append(
		append([]string{}, roles.Primary...),
		roles.Secondary...,
	)

	return uniqueStrings(combined)
}

func (s *Scorer) Score(
	j job.Job,
	c company.Company,
) Result {
	// Descriptions are stored as the source returned them, and Greenhouse
	// returns HTML that is itself HTML-escaped. Every text rule below is word
	// oriented, and an escaped tag welds itself to the word beside it: the
	// stored form of "8+ years" is "&lt;li&gt;8+ years", and "Go" inside
	// "&lt;strong&gt;Go&lt;/strong&gt;" is not the word "go".
	//
	// Scoring used to read that raw. The visible consequence was postings
	// demanding eight years scoring full marks on experience and reaching the
	// shortlist, because no figure could be parsed at all; skill and domain
	// matching were quietly degraded the same way. Normalise once, here, so
	// every rule reads the same plain text.
	description := job.NormalizeDescription(j.Description)

	text := strings.ToLower(
		strings.Join([]string{
			j.Title,
			description,
		}, " "),
	)

	candidateMatch := s.matcher.Match(
		text,
		s.preferences,
	)

	skills := scoreSkills(
		text,
		s.preferences.TechnologyPreferences,
	)

	candidateSkills := scoreCandidateSkills(
		candidateMatch,
	)

	role := scoreRole(
		j.Title,
		s.preferences.Roles,
		s.titleFamily,
	)

	requiredYears, requiredYearsFound := extractRequiredYears(
		j.Title,
		description,
	)

	experience := scoreExperience(
		requiredYears,
		requiredYearsFound,
		s.candidate.Experience.TotalYears,
	)

	domain := scoreDomain(
		text,
		s.preferences.Domains,
	)

	candidateDomain := scoreCandidateDomain(
		candidateMatch,
	)

	location := scoreLocation(
		j.Location,
		s.preferences.Locations,
	)

	companyScore := scoreCompany(
		c.Classification,
		s.preferences.CompanyType,
	)

	compensation := scoreCompensation(
		j.SalaryMin,
		j.SalaryMax,
		s.preferences.Compensation,
	)

	// Normalize over the signals actually present rather than a fixed 100.
	//
	// Absence of evidence was previously scored as mediocre evidence: a job with
	// no salary data still consumed the full 10-point compensation slot while
	// only ever earning 5. Greenhouse never publishes salary, so every job
	// sourced from it was capped roughly 5 points below what the thresholds
	// assume, and the same applied to an unclassified company and to
	// descriptions from which no required skills or domains could be parsed.
	//
	// Excluding an unavailable signal from both numerator and denominator keeps
	// the score comparable across sources, so thresholds stay meaningful.
	signals := []scoreSignal{
		{value: skills, max: maxSkillsScore, available: true},
		{
			value:     candidateSkills,
			max:       maxCandidateSkillsScore,
			available: candidateMatch.RequiredSkills > 0,
		},
		{value: role, max: maxRoleScore, available: true},
		{value: experience, max: maxExperienceScore, available: true},
		{value: domain, max: maxDomainScore, available: true},
		{
			value:     candidateDomain,
			max:       maxCandidateDomainScore,
			available: candidateMatch.RequiredDomains > 0,
		},
		{value: location, max: maxLocationScore, available: true},
		{
			value:     companyScore,
			max:       maxCompanyScore,
			available: c.Classification != company.ClassificationUnknown,
		},
		{
			value:     compensation,
			max:       maxCompensationScore,
			available: j.SalaryMin != nil || j.SalaryMax != nil,
		},
	}

	rawScore, maxRawScore := accumulateSignals(signals)

	overall := 0.0

	if maxRawScore > 0 {
		overall = (rawScore / maxRawScore) * 100.0
	}

	// Demotion penalty: a title on the demoted list (e.g. "Senior"/"Sr"/"Lead")
	// is not vetoed, but it ranks below an equivalent non-senior role. Applied
	// to the normalised score so a genuinely strong senior match can still clear
	// the shortlist threshold on its merits, while a marginal one drops out.
	// This is the "rank lower, stay visible" behaviour that mirrors how the job
	// boards' top-choices lists treat a slight seniority mismatch.
	demoted := containsAny(
		strings.ToLower(j.Title),
		s.preferences.Roles.Demoted,
	)

	if demoted {
		overall -= seniorityDemotionPenalty
		if overall < 0 {
			overall = 0
		}
	}

	// Name the excluded signals. Without this the breakdown still lists a value
	// for a component that did not count, which reads as though it contributed.
	excluded := excludedSignalNames(
		candidateMatch,
		c.Classification,
		j.SalaryMin,
		j.SalaryMax,
	)

	recommendation := RecommendationSkip

	switch {
	case overall >= s.preferences.JobQuality.MinimumScoreForApply:
		recommendation = RecommendationApply
	case overall >= s.preferences.JobQuality.MinimumScoreForShortlist:
		recommendation = RecommendationShortlist
	}

	// Veto only on a positive determination that this is a services company.
	//
	// Vetoing on anything that is not PRODUCT also rejected UNKNOWN, and UNKNOWN
	// is the norm rather than the exception: classification is derived from the
	// job descriptions stored for a company, and most companies contribute a
	// single posting. A genuine 71.2-scoring backend role was being silently
	// skipped purely because one description carried too little evidence.
	//
	// This also keeps the policy consistent with normalization, which already
	// excludes an unclassified company instead of penalizing it. Absence of
	// evidence is not evidence against.
	if s.preferences.CompanyType.RequireProductCompany &&
		c.Classification == company.ClassificationServices {
		recommendation = RecommendationSkip
	}

	// Hard title veto: only genuinely-unreachable roles and clear mismatches.
	// "Senior"/"Sr"/"Lead" no longer live here — they are demoted, not hidden,
	// because in India those titles are often 3-4 year roles the candidate can
	// legitimately apply to. Erasing them was the biggest reason recognised-good
	// roles never appeared.
	if containsAny(
		strings.ToLower(j.Title),
		s.preferences.Roles.Excluded,
	) {
		recommendation = RecommendationSkip
	}

	// A role demanding far more experience than the candidate has is the wrong
	// job, and a hard veto beyond a real ceiling still makes sense: "8+ years"
	// is not a stretch, it is a different candidate. But the previous behaviour
	// vetoed anything above the candidate's exact years, which hid every "3-4
	// years" posting. Now the ceiling is a genuine reach limit (default 5), and
	// the graded experienceScore already penalises the closer gaps, so a "4
	// years" role ranks lower but stays visible. The true required_years is
	// always printed in the reasoning, so the gap is never concealed.
	if requiredYearsFound && requiredYears > s.experienceHardCeiling() {
		recommendation = RecommendationSkip
	}

	// Same reasoning for geography. A San Francisco or Dublin role is not a
	// slightly worse Bangalore role, it is unreachable, so a low location score
	// that other components paper over is the wrong model. Veto on a positive
	// foreign signal.
	if isOutsidePreferredCountry(j.Location) {
		recommendation = RecommendationSkip
	}

	return Result{
		OverallScore:         overall,
		SkillsScore:          skills,
		CandidateSkillsScore: candidateSkills,
		RoleScore:            role,
		ExperienceScore:      experience,
		DomainScore:          domain,
		CandidateDomainScore: candidateDomain,
		LocationScore:        location,
		CompanyScore:         companyScore,
		CompensationScore:    compensation,
		Recommendation:       recommendation,
		Reasoning: fmt.Sprintf(
			"skills=%.1f candidate_skills=%.1f role=%.1f experience=%.1f domain=%.1f candidate_domain=%.1f location=%.1f company=%.1f compensation=%.1f candidate_skills_match=%d/%d candidate_domain_match=%d/%d excluded=[%s] normalized_over=%.1f required_years=%s ceiling_years=%.1f",
			skills,
			candidateSkills,
			role,
			experience,
			domain,
			candidateDomain,
			location,
			companyScore,
			compensation,
			candidateMatch.MatchedSkills,
			candidateMatch.RequiredSkills,
			candidateMatch.MatchedDomains,
			candidateMatch.RequiredDomains,
			strings.Join(excluded, ","),
			maxRawScore,
			formatRequiredYears(requiredYears, requiredYearsFound),
			s.experienceHardCeiling(),
		),
	}
}

// experienceHardCeiling is the veto line: a posting stating more years than this
// is forced to SKIP.
//
// This tracks the candidate's real experience rather than sitting above it. An
// earlier version deliberately set it higher so a "3-5 years" posting would stay
// visible with a reduced score, on the theory that the reader could judge the
// gap. That backfired — the shortlist filled with roles requiring three to eight
// years and the reader spent their time filtering by hand, which is the work the
// scorer exists to do.
//
// Ranges still count by their low end, so this is less blunt than it appears:
// "2-4 years" reads as 2 and survives for a two-year candidate.
//
// Falls back to the candidate's own total years when unset, and only to a
// constant if that is unknown too, so an absent config key tightens the filter
// rather than disabling it.
func (s *Scorer) experienceHardCeiling() float64 {
	if ceiling := s.preferences.ExperienceRequired.HardCeilingYears; ceiling > 0 {
		return ceiling
	}

	if years := s.candidate.Experience.TotalYears; years > 0 {
		return years
	}

	return defaultExperienceHardCeiling
}

// formatRequiredYears keeps "the posting stated nothing" distinguishable from
// "the posting stated zero" in the stored reasoning. Both are legitimate, and
// only one of them is ever vetoed.
func formatRequiredYears(years float64, found bool) string {
	if !found {
		return "none"
	}

	return fmt.Sprintf("%.1f", years)
}

// Component maximums. These must stay in sync with the corresponding score
// functions in rules.go, and are the denominator contributions used when a
// signal is present.
const (
	// Candidate-skill overlap is the dominant signal, mirroring how LinkedIn and
	// Naukri "top choices" rank: they reward how much of the posting's required
	// skills the candidate actually covers, far more than raw keyword presence.
	//
	// This is a deliberate inversion of the previous weighting, where raw
	// keyword counting (maxSkillsScore) outweighed real candidate coverage
	// (maxCandidateSkillsScore) two-to-one. That rewarded a job for merely
	// mentioning many skills, whether or not the candidate had them, and is why
	// JDs the candidate genuinely matched still failed to clear the threshold.
	//
	// maxSkillsScore is kept as a small booster for skill-dense postings rather
	// than removed, since a posting listing many relevant technologies is still
	// weak positive evidence.
	maxSkillsScore          = 10.0
	maxCandidateSkillsScore = 25.0
	maxRoleScore            = 15.0
	maxExperienceScore      = 15.0
	maxDomainScore          = 10.0
	maxCandidateDomainScore = 5.0
	maxLocationScore        = 10.0
	maxCompanyScore         = 5.0
	maxCompensationScore    = 10.0
)

const (
	// seniorityDemotionPenalty is the number of normalised score points a
	// demoted title (e.g. "Senior"/"Sr"/"Lead") loses. Large enough that a
	// marginal senior role drops below the shortlist threshold, small enough
	// that a genuinely strong senior match can still clear it on merit.
	seniorityDemotionPenalty = 12.0

	// defaultExperienceHardCeiling is the last-resort veto line, used only when
	// neither hard_ceiling_years nor the candidate's total_years is known.
	//
	// Deliberately low. The failure mode that matters is a filter that is too
	// loose: it fills the shortlist with roles the candidate cannot apply to and
	// pushes the filtering back onto them. Too strict merely produces a short
	// list, which is visible and easy to correct by raising the config value.
	defaultExperienceHardCeiling = 2.0
)

// scoreSignal is one scoring component together with whether the underlying
// evidence was actually available.
type scoreSignal struct {
	value     float64
	max       float64
	available bool
}

// accumulateSignals totals the available signals and the maximum they could have
// reached, so the caller can normalize over real evidence only.
func accumulateSignals(signals []scoreSignal) (float64, float64) {
	var total, maximum float64

	for _, signal := range signals {
		if !signal.available {
			continue
		}

		total += signal.value
		maximum += signal.max
	}

	return total, maximum
}

// excludedSignalNames lists the components left out of normalization because the
// evidence they depend on was not present in the job posting.
func excludedSignalNames(
	match CandidateMatch,
	classification company.Classification,
	salaryMin *int,
	salaryMax *int,
) []string {
	var excluded []string

	if match.RequiredSkills == 0 {
		excluded = append(excluded, "candidate_skills")
	}

	if match.RequiredDomains == 0 {
		excluded = append(excluded, "candidate_domain")
	}

	if classification == company.ClassificationUnknown {
		excluded = append(excluded, "company")
	}

	if salaryMin == nil && salaryMax == nil {
		excluded = append(excluded, "compensation")
	}

	return excluded
}
