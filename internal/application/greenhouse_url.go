package application

import (
	"fmt"
	"net/url"
	"strings"
)

func parseGreenhouseBoardToken(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse greenhouse job URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())

	switch host {
	case "boards.greenhouse.io", "job-boards.greenhouse.io":
	default:
		return "", fmt.Errorf(
			"unsupported greenhouse job host %q",
			host,
		)
	}

	parts := strings.Split(
		strings.Trim(parsed.Path, "/"),
		"/",
	)

	if len(parts) < 2 || parts[0] == "" || parts[1] != "jobs" {
		return "", fmt.Errorf(
			"invalid greenhouse job URL path %q",
			parsed.Path,
		)
	}

	return parts[0], nil
}
