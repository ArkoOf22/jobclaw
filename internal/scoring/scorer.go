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
	OverallScore      float64
	SkillsScore       float64
	RoleScore         float64
	ExperienceScore   float64
	DomainScore       float64
	LocationScore     float64
	CompanyScore      float64
	CompensationScore float64

	Recommendation Recommendation
	Reasoning      string
}

type Scorer struct {
	candidate   config.Candidate
	preferences config.JobPreferences
}

func NewScorer(
	candidate config.Candidate,
	preferences config.JobPreferences,
) *Scorer {
	return &Scorer{
		candidate:   candidate,
		preferences: preferences,
	}
}

func (s *Scorer) Score(j job.Job, c company.Company) Result {
	text := strings.ToLower(
		strings.Join([]string{
			j.Title,
			j.Description,
		}, " "),
	)

	skills := scoreSkills(
		text,
		s.preferences.TechnologyPreferences,
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

	overall := skills +
		role +
		experience +
		domain +
		location +
		companyScore +
		compensation

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

	if containsAny(strings.ToLower(j.Title), s.preferences.Roles.Excluded) {
		recommendation = RecommendationSkip
	}

	return Result{
		OverallScore:      overall,
		SkillsScore:       skills,
		RoleScore:         role,
		ExperienceScore:   experience,
		DomainScore:       domain,
		LocationScore:     location,
		CompanyScore:      companyScore,
		CompensationScore: compensation,
		Recommendation:    recommendation,
		Reasoning: fmt.Sprintf(
			"skills=%.1f role=%.1f experience=%.1f domain=%.1f location=%.1f company=%.1f compensation=%.1f",
			skills,
			role,
			experience,
			domain,
			location,
			companyScore,
			compensation,
		),
	}
}
