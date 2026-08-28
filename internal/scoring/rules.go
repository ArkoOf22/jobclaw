package scoring

import (
	"strconv"
	"strings"

	"jobclaw/internal/company"
	"jobclaw/internal/config"
)

func scoreSkills(
	text string,
	preferences config.TechnologyPreferences,
) float64 {
	const maxScore = 20.0

	strongMatches := countMatches(text, preferences.StronglyPreferred)
	preferredMatches := countMatches(text, preferences.Preferred)

	score := float64(strongMatches)*2.0 +
		float64(preferredMatches)*1.0

	if score > maxScore {
		return maxScore
	}

	return score
}

func scoreCandidateSkills(match CandidateMatch) float64 {
	const maxScore = 10.0

	if match.RequiredSkills == 0 {
		return 0
	}

	return maxScore * match.SkillMatchRatio()
}

func scoreRole(title string, roles config.Roles) float64 {
	if containsAny(title, roles.Excluded) {
		return 0
	}

	if containsAny(title, roles.Preferred) {
		return 15
	}

	if containsAny(title, roles.Acceptable) {
		return 10
	}

	return 3
}

func scoreExperience(text string, candidateYears float64) float64 {
	requiredMin, found := extractMinimumYears(text)

	if !found {
		return 15
	}

	switch {
	case requiredMin <= candidateYears:
		return 15
	case requiredMin <= candidateYears+1:
		return 10
	case requiredMin <= candidateYears+2:
		return 5
	default:
		return 0
	}
}

func scoreDomain(text string, domains config.Domains) float64 {
	const maxScore = 10.0

	strong := countMatches(text, domains.StronglyPreferred)
	preferred := countMatches(text, domains.Preferred)

	score := float64(strong)*3.0 +
		float64(preferred)*1.5

	if score > maxScore {
		return maxScore
	}

	return score
}

func scoreCandidateDomain(match CandidateMatch) float64 {
	const maxScore = 5.0

	if match.RequiredDomains == 0 {
		return 0
	}

	return maxScore * match.DomainMatchRatio()
}

func scoreLocation(
	location string,
	locations config.Locations,
) float64 {
	location = strings.ToLower(strings.TrimSpace(location))

	if location == "" {
		return 5
	}

	if containsAny(location, locations.Preferred) {
		return 10
	}

	if containsAny(location, locations.Acceptable) {
		return 6
	}

	return 2
}

func scoreCompany(
	classification company.Classification,
	preferences config.CompanyType,
) float64 {
	if classification == company.ClassificationProduct {
		return 5
	}

	if classification == company.ClassificationServices {
		return 0
	}

	return 2.5
}

func scoreCompensation(
	salaryMin *int,
	salaryMax *int,
	compensation config.Compensation,
) float64 {
	if salaryMin == nil && salaryMax == nil {
		return 5
	}

	var min, max float64

	if salaryMin != nil {
		min = float64(*salaryMin) / 100000
	}

	if salaryMax != nil {
		max = float64(*salaryMax) / 100000
	}

	if salaryMin != nil && salaryMax == nil {
		max = min
	}

	if salaryMin == nil {
		min = max
	}

	targetMin := compensation.TargetMinLPA
	targetMax := compensation.TargetMaxLPA

	if max >= targetMin && min <= targetMax {
		return 10
	}

	if max >= targetMin*0.85 {
		return 7
	}

	if max >= targetMin*0.70 {
		return 4
	}

	return 0
}

func countMatches(text string, terms []string) int {
	count := 0

	for _, term := range terms {
		term = strings.TrimSpace(term)

		if term == "" {
			continue
		}

		if containsTerm(text, term) {
			count++
		}
	}

	return count
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if containsTerm(text, term) {
			return true
		}
	}

	return false
}

// experienceStretchYears is how far above their own experience a candidate is
// assumed willing to reach. A two-year engineer is a realistic applicant for a
// "3-4 years" posting but not a "5+ years" one, so the veto fires above this.
const experienceStretchYears = 2.0

// exceedsCandidateExperience reports whether the posting explicitly demands
// substantially more experience than the candidate has.
//
// It fires only on an explicit figure in the text. A posting that states no
// requirement is deliberately not vetoed here: seniority implied by a title with
// no stated years is caught by the excluded-role list instead. Splitting the two
// keeps each rule simple and its reason legible.
func exceedsCandidateExperience(text string, candidateYears float64) bool {
	requiredMin, found := extractMinimumYears(text)
	if !found {
		return false
	}

	return requiredMin > candidateYears+experienceStretchYears
}

// indiaLocationTokens are the words that positively identify a role as being in
// India (or India-remote). Matched as whole words, so "ind" hits "Bangalore,
// IND" without also claiming "Indiana".
var indiaLocationTokens = []string{
	"india", "ind", "bengaluru", "bangalore", "hyderabad", "pune",
	"mumbai", "gurgaon", "gurugram", "delhi", "noida", "chennai",
	"kolkata", "ahmedabad", "karnataka", "telangana", "kerala",
	"kochi", "cochin", "trivandrum", "coimbatore", "jaipur", "indore",
	"nagpur", "chandigarh", "mysore", "mysuru", "vizag",
	"visakhapatnam", "thane", "nashik", "vadodara", "surat",
}

// foreignLocationTokens positively identify a role as being outside India. Kept
// to whole-word matches for the same reason: "us" must not match "Columbus".
var foreignLocationTokens = []string{
	"united states", "usa", "us", "canada", "ireland",
	"united kingdom", "uk", "singapore", "germany", "france",
	"netherlands", "australia", "japan", "china", "brazil", "mexico",
	"poland", "spain", "portugal", "philippines", "indonesia",
	"vietnam", "malaysia", "thailand", "emea", "americas", "apac",
	"europe", "toronto", "london", "dublin", "seattle", "chicago",
	"austin", "berlin", "amsterdam", "sydney", "dubai", "uae",
	"new york", "san francisco",
}

// isOutsidePreferredCountry vetoes a role the candidate cannot take because it is
// not in India.
//
// An India signal always wins first, so "Bengaluru, India (US shift)" or
// "Bangalore, IND; Remote US" stays. After that:
//
//   - A remote role with no India signal is vetoed. A denylist of countries can
//     never be complete ("Remote - Estonia" is the case that proved it), and a
//     remote posting that does not say India is, for this candidate, not India.
//     Genuine India-remote roles almost always say so ("Remote, India").
//   - A non-remote role is vetoed only on an explicit foreign signal, since an
//     unrecognised onsite city with no country is more likely a smaller Indian
//     city the allowlist does not name than a foreign one, given the search is
//     already constrained to India.
//
// An empty location is never vetoed: absence of evidence is not evidence.
func isOutsidePreferredCountry(location string) bool {
	location = strings.TrimSpace(location)
	if location == "" {
		return false
	}

	if containsAny(location, indiaLocationTokens) {
		return false
	}

	if strings.Contains(strings.ToLower(location), "remote") {
		return true
	}

	return containsAny(location, foreignLocationTokens)
}

func extractMinimumYears(text string) (float64, bool) {
	words := strings.Fields(text)

	for i := range words {
		if i+1 >= len(words) {
			continue
		}

		if !strings.Contains(strings.ToLower(words[i+1]), "year") {
			continue
		}

		value := strings.Trim(
			words[i],
			"()[]{}:;,",
		)

		value = strings.TrimSuffix(value, "+")
		value = strings.ReplaceAll(value, "–", "-")

		if idx := strings.Index(value, "-"); idx >= 0 {
			value = value[:idx]
		}

		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}

		return n, true
	}

	return 0, false
}
