package application

import (
	"html"
	"regexp"
	"strings"
)

// Job descriptions arrive as HTML, and Greenhouse returns it HTML-escaped, so a
// single "<" is stored as "&lt;" and a non-breaking space as "&amp;nbsp;" —
// eleven characters for one space. Sent raw, the model is billed to read escaped
// markup that carries no meaning.
//
// Normalising is a pure saving: unescaping and stripping tags removes no
// information, and a cleaner prompt is easier for the model to follow.

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

// NormalizeJobDescription converts a stored description into plain text.
//
// Entities are unescaped twice: Greenhouse escapes markup that already contains
// entities, so "&amp;nbsp;" needs two passes to become a space.
func NormalizeJobDescription(raw string) string {
	text := raw

	// Two passes, since the stored form is escaped HTML containing entities.
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

// boilerplatePrefixes open sections that describe the employer rather than the
// role. They are identical across every posting from a company and contribute
// nothing to tailoring, which only needs to know what the job requires.
var boilerplatePrefixes = []string{
	"who we are",
	// Covers "About Stripe", "About us", "About the team". Real postings head
	// their company and team blurbs this way, and matching only "about us" left
	// several thousand characters of it in place. Role-specific "about"
	// headings are excluded below.
	"about",
	"our mission",
	"why join",
	"life at",
	"benefits",
	"perks",
	"pay and benefits",
	"compensation and benefits",
	"equal opportunity",
	"equal employment",
	"eeo",
	"diversity",
	"accommodations",
	"privacy notice",
	"applicant privacy",
	"office locations",
	"in-office expectations",
	"hybrid work",
}

// TrimJobDescription drops employer boilerplate and caps the remainder.
//
// Conservative by design: a paragraph is dropped only when its opening line
// matches a known boilerplate heading, and the cap is applied at a paragraph
// boundary so a requirement is never cut mid-sentence. If trimming would remove
// nearly everything the original is returned instead, since a short or oddly
// structured description is better sent whole than gutted.
func TrimJobDescription(text string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 6000
	}

	// Drop whole sections, not individual paragraphs.
	//
	// Stripping tags turns an HTML heading into its own short paragraph, so the
	// prose beneath it becomes a separate paragraph that carries no boilerplate
	// keyword of its own. Dropping only the heading would leave the entire
	// "About Stripe" body behind, which is most of the waste. A boilerplate
	// heading therefore suppresses everything until the next heading.
	paragraphs := strings.Split(text, "\n\n")
	kept := make([]string, 0, len(paragraphs))

	skipping := false

	for _, paragraph := range paragraphs {
		trimmed := strings.TrimSpace(paragraph)

		if trimmed == "" {
			continue
		}

		heading := looksLikeHeading(trimmed)

		switch {
		case isBoilerplate(trimmed):
			// Boilerplate heading, or a paragraph opening with one.
			skipping = heading

			continue

		case heading:
			// A non-boilerplate heading ends any suppressed section.
			skipping = false

		case skipping:
			// Body text belonging to a suppressed section.
			continue
		}

		kept = append(kept, trimmed)
	}

	result := strings.Join(kept, "\n\n")

	// Guard against over-trimming: if almost nothing survived, the heading
	// heuristics have misfired on this posting's structure.
	if len(result) < len(text)/4 {
		result = text
	}

	if len(result) <= maxChars {
		return result
	}

	// Cut at a paragraph boundary rather than mid-sentence.
	var builder strings.Builder

	for _, paragraph := range strings.Split(result, "\n\n") {
		if builder.Len()+len(paragraph)+2 > maxChars {
			break
		}

		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}

		builder.WriteString(paragraph)
	}

	if builder.Len() == 0 {
		// A single paragraph longer than the cap: take a hard prefix.
		return strings.TrimSpace(result[:maxChars])
	}

	return builder.String()
}

// isBoilerplate reports whether a paragraph opens with an employer-boilerplate
// heading. Only the first line is examined, so a requirement that happens to
// mention benefits is not discarded.
//
// Once tags are stripped, an HTML heading becomes a standalone short paragraph,
// so the heading and the prose beneath it end up as separate paragraphs. A bare
// heading is therefore dropped on its own, and the following paragraph is judged
// on its own opening text.
func isBoilerplate(paragraph string) bool {
	firstLine := paragraph

	if index := strings.IndexByte(paragraph, '\n'); index >= 0 {
		firstLine = paragraph[:index]
	}

	firstLine = strings.ToLower(strings.TrimSpace(firstLine))

	// Long opening lines are prose, not headings, so a keyword match would be
	// incidental rather than structural.
	if len(firstLine) > 60 {
		return false
	}

	firstLine = strings.Trim(firstLine, " :–-—#*")

	// "About the role" and friends describe the job itself, so they are the one
	// family of "about" headings worth keeping.
	for _, keep := range []string{
		"about the role",
		"about this role",
		"about the job",
		"about the position",
		"about the opportunity",
	} {
		if strings.HasPrefix(firstLine, keep) {
			return false
		}
	}

	for _, prefix := range boilerplatePrefixes {
		if strings.HasPrefix(firstLine, prefix) {
			return true
		}
	}

	return false
}

// looksLikeHeading reports whether a paragraph is a section heading rather than
// body text.
//
// After tag stripping there is no markup left to identify headings, so this uses
// shape: headings are short, single-line, and not sentences. Bullet lines are
// excluded because a list item is content, not a section boundary.
func looksLikeHeading(paragraph string) bool {
	if strings.Contains(paragraph, "\n") {
		return false
	}

	trimmed := strings.TrimSpace(paragraph)

	if trimmed == "" || len(trimmed) > 60 {
		return false
	}

	// List markers indicate content.
	for _, marker := range []string{"-", "*", "•", "◦", "·"} {
		if strings.HasPrefix(trimmed, marker) {
			return false
		}
	}

	// A terminal full stop, question mark, comma or semicolon means prose.
	switch trimmed[len(trimmed)-1] {
	case '.', '?', ',', ';':
		return false
	}

	return true
}
