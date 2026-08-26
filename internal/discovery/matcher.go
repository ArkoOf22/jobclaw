package discovery

import (
	"strings"
	"unicode"

	"jobclaw/internal/job"
)

func MatchesRequest(
	candidate job.Job,
	request Request,
) bool {
	if !matchesKeywords(candidate, request.Keywords) {
		return false
	}

	if !matchesLocations(candidate, request.Locations) {
		return false
	}

	if request.RemoteOnly && !isRemote(candidate) {
		return false
	}

	return true
}

func matchesKeywords(
	candidate job.Job,
	keywords []string,
) bool {
	if len(keywords) == 0 {
		return true
	}

	title := normalizeSearchText(candidate.Title)
	description := normalizeSearchText(candidate.Description)

	hasKeyword := false

	for _, keyword := range keywords {
		keyword = normalizeSearchText(keyword)
		if keyword == "" {
			continue
		}

		hasKeyword = true
		tokens := strings.Fields(keyword)

		// Single-word terms are usually skills or technologies.
		// They may appear in either the job title or description.
		if len(tokens) == 1 {
			if strings.Contains(title, keyword) ||
				strings.Contains(description, keyword) {
				return true
			}

			continue
		}

		// Multi-word terms represent role intent. Require the
		// complete phrase, or all meaningful tokens, to match
		// the JOB TITLE. Do not match them from an incidental
		// mention inside the description.
		if strings.Contains(title, keyword) {
			return true
		}

		allTokensMatch := true
		meaningfulTokens := 0

		for _, token := range tokens {
			if len(token) < 3 {
				continue
			}

			meaningfulTokens++

			if !strings.Contains(title, token) {
				allTokensMatch = false
				break
			}
		}

		if meaningfulTokens > 0 && allTokensMatch {
			return true
		}
	}

	return !hasKeyword
}

func matchesLocations(
	candidate job.Job,
	locations []string,
) bool {
	if len(locations) == 0 {
		return true
	}

	location := strings.ToLower(
		strings.TrimSpace(candidate.Location),
	)

	if location == "" {
		return false
	}

	// A remote role is allowed for any preferred location.
	if strings.Contains(location, "remote") {
		return true
	}

	for _, preferred := range locations {
		preferred = strings.ToLower(
			strings.TrimSpace(preferred),
		)

		if preferred == "" {
			continue
		}

		if strings.Contains(location, preferred) ||
			strings.Contains(preferred, location) {
			return true
		}

		for _, token := range strings.Fields(preferred) {
			if len(token) >= 3 &&
				strings.Contains(location, token) {
				return true
			}
		}
	}

	return false
}

func isRemote(candidate job.Job) bool {
	searchable := strings.ToLower(
		candidate.Title + " " + candidate.Location,
	)

	return strings.Contains(searchable, "remote")
}

func normalizeSearchText(value string) string {
	value = strings.ToLower(value)

	var builder strings.Builder
	builder.Grow(len(value))

	for _, r := range value {
		if unicode.IsLetter(r) ||
			unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}

		builder.WriteRune(' ')
	}

	return strings.Join(
		strings.Fields(builder.String()),
		" ",
	)
}
