package scoring

import (
	"strings"

	"jobclaw/internal/config"
)

type CandidateMatch struct {
	MatchedSkills   int
	RequiredSkills  int
	MatchedDomains  int
	RequiredDomains int

	Skills  []string
	Domains []string
}

func (m CandidateMatch) SkillMatchRatio() float64 {
	if m.RequiredSkills == 0 {
		return 1
	}

	return float64(m.MatchedSkills) / float64(m.RequiredSkills)
}

func (m CandidateMatch) DomainMatchRatio() float64 {
	if m.RequiredDomains == 0 {
		return 1
	}

	return float64(m.MatchedDomains) / float64(m.RequiredDomains)
}

type CandidateMatcher struct {
	candidate config.Candidate
}

func NewCandidateMatcher(candidate config.Candidate) *CandidateMatcher {
	return &CandidateMatcher{
		candidate: candidate,
	}
}

func (m *CandidateMatcher) Match(
	text string,
	preferences config.JobPreferences,
) CandidateMatch {
	text = strings.ToLower(text)

	candidateSkills := flattenCandidateSkills(m.candidate.Skills)
	candidateDomains := uniqueStrings(m.candidate.DomainExperience)

	preferredSkills := uniqueStrings(append(
		append(
			[]string{},
			preferences.TechnologyPreferences.StronglyPreferred...,
		),
		preferences.TechnologyPreferences.Preferred...,
	))

	preferredDomains := uniqueStrings(append(
		append(
			[]string{},
			preferences.Domains.StronglyPreferred...,
		),
		preferences.Domains.Preferred...,
	))

	var result CandidateMatch

	for _, skill := range preferredSkills {
		if !containsTerm(text, skill) {
			continue
		}

		result.RequiredSkills++

		if containsTermInList(candidateSkills, skill) {
			result.MatchedSkills++
			result.Skills = append(result.Skills, skill)
		}
	}

	for _, domain := range preferredDomains {
		if !containsTerm(text, domain) {
			continue
		}

		result.RequiredDomains++

		if containsTermInList(candidateDomains, domain) {
			result.MatchedDomains++
			result.Domains = append(result.Domains, domain)
		}
	}

	return result
}

func flattenCandidateSkills(skills config.Skills) []string {
	return uniqueStrings(append(
		append(
			append(
				append(
					append(
						append(
							[]string{},
							skills.Languages...,
						),
						skills.Backend...,
					),
					skills.Databases...,
				),
				skills.CloudInfrastructure...,
			),
			skills.Observability...,
		),
		skills.AIEngineering...,
	))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)

		if value == "" {
			continue
		}

		key := strings.ToLower(value)

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, value)
	}

	return result
}

func containsTermInList(values []string, term string) bool {
	for _, value := range values {
		if containsTerm(value, term) || containsTerm(term, value) {
			return true
		}
	}

	return false
}

func containsTerm(text, term string) bool {
	text = strings.ToLower(text)
	term = strings.ToLower(strings.TrimSpace(term))

	if term == "" {
		return false
	}

	normalizedText := strings.NewReplacer(
		"-", " ",
		"/", " ",
		",", " ",
		".", " ",
		"(", " ",
		")", " ",
	).Replace(text)

	normalizedTerm := strings.NewReplacer(
		"-", " ",
		"/", " ",
		",", " ",
		".", " ",
		"(", " ",
		")", " ",
	).Replace(term)

	words := strings.Fields(normalizedText)
	termWords := strings.Fields(normalizedTerm)

	if len(termWords) == 0 {
		return false
	}

	if len(termWords) == 1 {
		for _, word := range words {
			if word == termWords[0] {
				return true
			}

			if strings.HasSuffix(termWords[0], "s") &&
				!strings.HasSuffix(termWords[0], "ss") &&
				word == strings.TrimSuffix(termWords[0], "s") {
				return true
			}

			if strings.HasSuffix(termWords[0], "ies") &&
				word == strings.TrimSuffix(termWords[0], "ies")+"y" {
				return true
			}
		}

		return false
	}

	for i := 0; i <= len(words)-len(termWords); i++ {
		match := true

		for j := range termWords {
			if words[i+j] != termWords[j] {
				match = false
				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
