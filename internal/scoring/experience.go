package scoring

import (
	"regexp"
	"strconv"
	"strings"
)

// Reading an experience requirement out of a job posting.
//
// This is a veto input, not just a score input: a posting demanding more years
// than the candidate has is the wrong job, not a weak match, and a strong
// stack-and-domain fit will otherwise carry it over the shortlist threshold.
// Three separate holes let that happen, all closed here.
//
//  1. Descriptions are stored as escaped HTML. The old reader split on
//     whitespace and required the token before "years" to parse as a number, so
//     "&lt;li&gt;8+ years of experience" yielded nothing. Every Greenhouse
//     posting therefore read as stating no requirement at all, which scored full
//     marks on experience and skipped the veto entirely. Callers now pass
//     normalised plain text.
//
//  2. Only the literal word "year" was recognised. "_4+Yrs_" in a title, a
//     common Naukri/JobSpy shape, read as no requirement.
//
//  3. Only the first mention anywhere in the text was read. A posting listing
//     several bars has to be judged on the highest of them, because the
//     candidate must satisfy all of them.
//
// The veto itself lives in Scorer.Score rather than here, because it needs the
// same parsed figure the experience score does and parsing the description twice
// invites the two to disagree.
//
// A posting that states no requirement at all is deliberately not vetoed:
// seniority implied by a title with no stated years is caught by the
// excluded-role list instead. Splitting the two keeps each rule legible.

// yearsPhrasePattern matches a stated span of years: "3+ years", "2-4 years",
// "5 to 7 yrs", "1.00 + years", "3 Year(s)".
//
// The second number is captured but deliberately unused. In a range the low end
// is the entry bar, and a candidate at the low end is a genuine applicant.
var yearsPhrasePattern = regexp.MustCompile(
	`(\d{1,2}(?:\.\d+)?)\s*\+?\s*` +
		`(?:(?:-|to|or)\s*(\d{1,2}(?:\.\d+)?)\s*\+?\s*)?` +
		`(?:years?|yrs?)\b`,
)

// experienceContextTerms mark a years figure as a statement about how much
// experience the role requires, rather than an incidental mention such as
// "founded 12 years ago" or "grown 10x in 3 years".
//
// An allowlist rather than a denylist: a figure with no requirement language
// near it is far more likely to be prose than a hidden bar, and a wrong veto
// silently hides a job the candidate wanted to see. Titles bypass this check —
// a job title that names years is naming the requirement.
var experienceContextTerms = []string{
	"experience", "expertise", "background", "exp:", "exp.",
	"hands on", "hands-on", "professional", "industry",
	"relevant", "minimum", "min:", "min.", "atleast", "at least",
	"must have", "must-have", "should have", "requirement",
	"qualification", "looking for", "seeking", "mandate",
	"mandatory", "you bring", "you have", "we expect",
	"proven", "track record",
}

// experienceExclusionSuffixes disqualify a figure that describes schooling
// rather than a career. "15 years full time education" is a standard line in
// Indian job descriptions and would otherwise veto every posting carrying it.
var experienceExclusionSuffixes = []string{
	"education", "schooling", "academic", "of study",
}

const (
	// Window sizes around a match, in bytes. Tight enough that requirement
	// language from a neighbouring sentence does not bleed in, wide enough for
	// the usual phrasings ("Key Skills And Experience * 3-5 years of ...").
	experienceContextBefore = 60
	experienceContextAfter  = 80

	// The exclusion window is far tighter than the context window, and
	// deliberately so. These postings run the two lines together:
	//
	//	Minimum 3 Year(s) Of Experience Is Required
	//	Educational Qualification : 15 years full time education
	//
	// Sharing the 80-byte window let "Educational" from the second line
	// suppress the real requirement on the first, which is how "Minimum 5
	// Year(s) Of Experience" postings survived. Schooling language always sits
	// directly against its own figure, so a short window separates the two.
	experienceExclusionAfter = 25
)

// extractRequiredYears reports the number of years of experience a posting
// demands, and whether it stated any figure at all.
//
// When several figures are stated the highest is returned. A posting asking for
// "3-5 years of backend" and "8+ years of distributed systems" requires both, so
// the binding bar is the larger one. Within a single range the low end is used,
// since "2-4 years" is genuinely open to a two-year candidate.
//
// title and description are read separately because they carry different
// evidential weight, not for convenience: years in a title are always the
// requirement, years in a description body need supporting language.
func extractRequiredYears(title, description string) (float64, bool) {
	required := 0.0
	found := false

	if years, ok := scanYears(title, false); ok {
		required = years
		found = true
	}

	if years, ok := scanYears(description, true); ok {
		if !found || years > required {
			required = years
		}

		found = true
	}

	return required, found
}

// scanYears returns the highest stated requirement in text. When requireContext
// is set, a figure only counts if requirement language sits near it.
func scanYears(text string, requireContext bool) (float64, bool) {
	text = normalizeYearsText(text)

	highest := 0.0
	found := false

	for _, match := range yearsPhrasePattern.FindAllStringSubmatchIndex(text, -1) {
		lowEnd := text[match[2]:match[3]]

		years, err := strconv.ParseFloat(lowEnd, 64)
		if err != nil {
			continue
		}

		before := window(text, match[0]-experienceContextBefore, match[0])
		after := window(text, match[1], match[1]+experienceContextAfter)

		if hasAnySubstring(
			window(text, match[1], match[1]+experienceExclusionAfter),
			experienceExclusionSuffixes,
		) {
			continue
		}

		if requireContext &&
			!hasAnySubstring(before, experienceContextTerms) &&
			!hasAnySubstring(after, experienceContextTerms) {
			continue
		}

		if !found || years > highest {
			highest = years
		}

		found = true
	}

	return highest, found
}

// normalizeYearsText lowercases and flattens the separators that would
// otherwise hide a figure from the pattern.
//
// Underscores matter specifically: Go treats "_" as a word character, so the
// trailing \b in the pattern fails on "4+Yrs_Bangalore" unless the underscore
// becomes a space first. Unicode dashes are folded to "-" so a single range
// alternative covers them all.
func normalizeYearsText(text string) string {
	return strings.NewReplacer(
		"_", " ",
		"|", " ",
		"\u2013", "-",
		"\u2014", "-",
		"\u2012", "-",
		"\u2212", "-",
	).Replace(strings.ToLower(text))
}

// window returns text[from:to] clamped to the bounds of text.
func window(text string, from, to int) string {
	if from < 0 {
		from = 0
	}

	if to > len(text) {
		to = len(text)
	}

	if from >= to {
		return ""
	}

	return text[from:to]
}

func hasAnySubstring(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}

	return false
}
