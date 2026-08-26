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
}

func NewScorer(
	candidate config.Candidate,
	preferences config.JobPreferences,
) *Scorer {
	return &Scorer{
		candidate:   candidate,
		preferences: preferences,
		matcher:     NewCandidateMatcher(candidate),
	}
}

func (s *Scorer) Score(
	j job.Job,
	c company.Company,
) Result {
	text := strings.ToLower(
		strings.Join([]string{
			j.Title,
			j.Description,
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
	)

	experience := scoreExperience(
		text,
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

	if s.preferences.CompanyType.RequireProductCompany &&
		c.Classification != company.ClassificationProduct {
		recommendation = RecommendationSkip
	}

	if containsAny(
		strings.ToLower(j.Title),
		s.preferences.Roles.Excluded,
	) {
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
			"skills=%.1f candidate_skills=%.1f role=%.1f experience=%.1f domain=%.1f candidate_domain=%.1f location=%.1f company=%.1f compensation=%.1f candidate_skills_match=%d/%d candidate_domain_match=%d/%d excluded=[%s] normalized_over=%.1f",
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
		),
	}
}

// Component maximums. These must stay in sync with the corresponding score
// functions in rules.go, and are the denominator contributions used when a
// signal is present.
const (
	maxSkillsScore          = 20.0
	maxCandidateSkillsScore = 10.0
	maxRoleScore            = 15.0
	maxExperienceScore      = 15.0
	maxDomainScore          = 10.0
	maxCandidateDomainScore = 5.0
	maxLocationScore        = 10.0
	maxCompanyScore         = 5.0
	maxCompensationScore    = 10.0
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
