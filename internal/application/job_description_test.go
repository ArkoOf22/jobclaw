package application

import (
	"strings"
	"testing"
)

// Greenhouse stores descriptions as escaped HTML, so entities need two passes:
// the markup itself is escaped and already contains entities.
func TestNormalizeJobDescriptionUnescapesDoubleEncodedHTML(t *testing.T) {
	raw := "&lt;h2&gt;Who we are&lt;/h2&gt;" +
		"&lt;p&gt;Stripe is a financial infrastructure platform." +
		"&amp;nbsp;We&#39;re hiring.&lt;/p&gt;"

	got := NormalizeJobDescription(raw)

	for _, unwanted := range []string{
		"&lt;", "&gt;", "&amp;", "&#39;", "<h2>", "</p>", "nbsp",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("output still contains %q: %q", unwanted, got)
		}
	}

	if !strings.Contains(got, "Who we are") {
		t.Fatalf("heading text lost: %q", got)
	}

	if !strings.Contains(got, "We're hiring.") {
		t.Fatalf("apostrophe entity not decoded: %q", got)
	}
}

// List items must stay on separate lines. Stripping tags without marking block
// boundaries first would run every requirement into one line.
func TestNormalizeJobDescriptionKeepsListItemsSeparate(t *testing.T) {
	raw := "&lt;ul&gt;&lt;li&gt;Go and Java&lt;/li&gt;" +
		"&lt;li&gt;Kafka and gRPC&lt;/li&gt;&lt;/ul&gt;"

	got := NormalizeJobDescription(raw)

	if !strings.Contains(got, "Go and Java") ||
		!strings.Contains(got, "Kafka and gRPC") {
		t.Fatalf("list content lost: %q", got)
	}

	if strings.Contains(got, "Go and JavaKafka") {
		t.Fatalf("list items merged: %q", got)
	}
}

func TestNormalizeJobDescriptionDropsScriptAndStyle(t *testing.T) {
	raw := "&lt;p&gt;Requirements&lt;/p&gt;" +
		"&lt;script&gt;var tracking = 1;&lt;/script&gt;" +
		"&lt;style&gt;.a{color:red}&lt;/style&gt;"

	got := NormalizeJobDescription(raw)

	if strings.Contains(got, "tracking") || strings.Contains(got, "color:red") {
		t.Fatalf("script or style body survived: %q", got)
	}

	if !strings.Contains(got, "Requirements") {
		t.Fatalf("real content lost: %q", got)
	}
}

// Employer boilerplate is identical across a company's postings and adds nothing
// to tailoring, while requirements must always survive.
func TestTrimJobDescriptionDropsBoilerplateKeepsRequirements(t *testing.T) {
	text := strings.Join([]string{
		"Who we are\nStripe is a financial infrastructure platform.",
		"Minimum requirements\nFour years of backend experience with Go.",
		"Benefits\nHealth insurance and equity.",
		"Preferred qualifications\nExperience with Kafka and gRPC.",
		"Equal opportunity\nWe are an equal opportunity employer.",
	}, "\n\n")

	got := TrimJobDescription(text, 6000)

	for _, want := range []string{
		"Minimum requirements",
		"Four years of backend experience",
		"Preferred qualifications",
		"Kafka and gRPC",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("dropped required content %q from: %q", want, got)
		}
	}

	for _, unwanted := range []string{
		"Who we are",
		"Health insurance",
		"equal opportunity employer",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("boilerplate %q survived: %q", unwanted, got)
		}
	}
}

// A requirement that merely mentions benefits must not be discarded. Only a
// paragraph's opening heading is examined.
func TestTrimJobDescriptionKeepsRequirementsMentioningBenefits(t *testing.T) {
	text := "Requirements\nExperience building benefits and payroll systems " +
		"at scale, including equal opportunity reporting pipelines."

	got := TrimJobDescription(text, 6000)

	if !strings.Contains(got, "payroll systems") {
		t.Fatalf("requirement dropped on an incidental keyword: %q", got)
	}
}

// If the heuristics misfire on an unusual structure, the original is safer than
// a gutted description.
func TestTrimJobDescriptionFallsBackWhenOverTrimming(t *testing.T) {
	text := "About us\nWe are a company.\n\nBenefits\nGood ones.\n\n" +
		"Perks\nMany."

	got := TrimJobDescription(text, 6000)

	if got != text {
		t.Fatalf(
			"expected the original when trimming removes nearly everything, got: %q",
			got,
		)
	}
}

// The cap must land on a paragraph boundary so a requirement is never cut
// mid-sentence.
func TestTrimJobDescriptionCutsAtParagraphBoundary(t *testing.T) {
	long := strings.Repeat("Requirement paragraph text here.", 20)

	text := strings.Join([]string{
		"Requirements\n" + long,
		"More requirements\n" + long,
		"Even more\n" + long,
	}, "\n\n")

	got := TrimJobDescription(text, 800)

	if len(got) > 800 {
		t.Fatalf("length = %d, want <= 800", len(got))
	}

	if strings.HasSuffix(got, "Requirement paragraph text her") {
		t.Fatal("cut mid-sentence rather than at a paragraph boundary")
	}
}

func TestTrimJobDescriptionHandlesSingleOversizedParagraph(t *testing.T) {
	text := "Requirements " + strings.Repeat("x", 2000)

	got := TrimJobDescription(text, 500)

	if len(got) > 500 {
		t.Fatalf("length = %d, want <= 500", len(got))
	}

	if got == "" {
		t.Fatal("expected a hard prefix rather than an empty result")
	}
}
