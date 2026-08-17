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

	// Component scores add up to a maximum of 110.
	// Normalize the final score to a 0-100 scale so that
	// JobQuality thresholds remain intuitive.
	rawScore := skills +
		candidateSkills +
		role +
		experience +
		domain +
		candidateDomain +
		location +
		companyScore +
		compensation

	const maxRawScore = 100.0
	overall := (rawScore / maxRawScore) * 100.0

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
			"skills=%.1f candidate_skills=%.1f role=%.1f experience=%.1f domain=%.1f candidate_domain=%.1f location=%.1f company=%.1f compensation=%.1f candidate_skills_match=%d/%d candidate_domain_match=%d/%d",
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
		),
	}
}
