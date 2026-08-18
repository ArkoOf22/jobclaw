package application

import (
	"fmt"
	"regexp"
	"strings"
)

type ResumeFactGuard struct {
	masterResume string
}

func NewResumeFactGuard(masterResume string) *ResumeFactGuard {
	return &ResumeFactGuard{
		masterResume: masterResume,
	}
}

func (g *ResumeFactGuard) Validate(generated string) error {
	if strings.TrimSpace(g.masterResume) == "" {
		return fmt.Errorf("master resume is required")
	}

	if strings.TrimSpace(generated) == "" {
		return fmt.Errorf("generated resume is empty")
	}

	if strings.Contains(generated, "```") {
		return fmt.Errorf("generated resume contains code fence")
	}

	lower := strings.ToLower(generated)

	for _, phrase := range []string{
		"here is your tailored resume",
		"here's your tailored resume",
		"i tailored your resume",
		"as an ai",
		"generation commentary",
	} {
		if strings.Contains(lower, phrase) {
			return fmt.Errorf("generated resume contains unsupported commentary")
		}
	}

	for _, metric := range extractMetrics(generated) {
		if !metricExists(g.masterResume, metric) {
			return fmt.Errorf(
				"generated resume contains unsupported metric %q",
				metric,
			)
		}
	}

	return nil
}

func extractMetrics(text string) []string {
	re := regexp.MustCompile(
		`(?i)(?:\d+(?:\.\d+)?\s*(?:%|Cr|GB|M|K|days?|records?/day|requests?/sec))`,
	)

	matches := re.FindAllString(text, -1)

	result := make([]string, 0, len(matches))
	seen := make(map[string]struct{})

	for _, match := range matches {
		normalized := strings.TrimSpace(match)

		key := strings.ToLower(normalized)

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func metricExists(masterResume, metric string) bool {
	master := strings.ToLower(masterResume)
	target := strings.ToLower(strings.TrimSpace(metric))

	return strings.Contains(master, target)
}
