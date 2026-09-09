package job

import (
	"html"
	"regexp"
	"strings"
)

// Descriptions are stored exactly as the source returned them, and Greenhouse
// returns HTML that is itself HTML-escaped: a single "<" is stored as "&lt;" and
// a non-breaking space as "&amp;nbsp;".
//
// That storage format has to be undone before any text rule reads the
// description. Word-oriented rules see "&lt;li&gt;8+" instead of "8+", so a
// posting demanding "8+ years of experience" reads as a posting stating no
// requirement at all. Normalising lives here, in the job domain, because every
// consumer of a description needs it, not just the ones building LLM prompts.

var (
	htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

	// Script and style bodies carry no job information.
	scriptStylePattern = regexp.MustCompile(
		`(?is)<(script|style)[^>]*>.*?</(script|style)>`,
	)

	// Block-level tags become paragraph breaks so list items stay separate
	// rather than running together into one line.
	blockBreakPattern = regexp.MustCompile(
		`(?i)</(p|div|li|ul|ol|h[1-6]|tr|table|section)>|<br\s*/?>`,
	)

	excessBlankLines = regexp.MustCompile(`\n{3,}`)
	trailingSpaces   = regexp.MustCompile(`[ \t]+\n`)
	repeatedSpaces   = regexp.MustCompile(`[ \t]{2,}`)
)

// NormalizeDescription converts a stored description into plain text.
//
// Entities are unescaped twice: the stored form is escaped HTML that already
// contained entities, so "&amp;nbsp;" needs two passes to become a space.
func NormalizeDescription(raw string) string {
	text := raw

	text = html.UnescapeString(text)
	text = html.UnescapeString(text)

	text = scriptStylePattern.ReplaceAllString(text, " ")

	// Mark block boundaries before tags are removed, otherwise list items and
	// paragraphs merge into a single unreadable line.
	//
	// A blank line, not a single newline: downstream trimming works on
	// paragraphs, and a single newline would leave the whole description as one
	// paragraph that no heading rule could ever match.
	text = blockBreakPattern.ReplaceAllString(text, "\n\n")

	text = htmlTagPattern.ReplaceAllString(text, " ")

	// Non-breaking spaces survive unescaping as U+00A0.
	text = strings.ReplaceAll(text, "\u00a0", " ")

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	text = repeatedSpaces.ReplaceAllString(text, " ")
	text = trailingSpaces.ReplaceAllString(text, "\n")
	text = excessBlankLines.ReplaceAllString(text, "\n\n")

	// Tag removal leaves a leading space on most lines, which would defeat
	// prefix matching downstream.
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	text = strings.Join(lines, "\n")
	text = excessBlankLines.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
