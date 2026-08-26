package sheet

import (
	"testing"
	"time"
)

// Column order is the sheet's contract. Existing rows would no longer line up
// with their headers if the order changed, so this pins it.
func TestRowCellsMatchHeaderOrder(t *testing.T) {
	discovered := time.Date(2026, 8, 26, 14, 30, 0, 0, time.UTC)

	row := Row{
		JobID:          473,
		Score:          "77.3",
		Recommendation: "SHORTLIST",
		Company:        "Stripe",
		Title:          "Software Engineer, Internal Systems",
		Location:       "Bengaluru, India",
		Source:         "greenhouse",
		ApplyURL:       "https://stripe.com/jobs/search?gh_jid=7543868",
		ResumeLink:     "",
		Status:         "NEW",
		Reasoning:      "skills=9.0 role=15.0",
		DiscoveredAt:   discovered,
	}

	cells := row.cells()
	header := Header()

	if len(cells) != len(header) {
		t.Fatalf(
			"cells = %d, header = %d; they must stay aligned",
			len(cells),
			len(header),
		)
	}

	expected := []string{
		"473",
		"77.3",
		"SHORTLIST",
		"Stripe",
		"Software Engineer, Internal Systems",
		"Bengaluru, India",
		"greenhouse",
		"https://stripe.com/jobs/search?gh_jid=7543868",
		"",
		"NEW",
		"skills=9.0 role=15.0",
		"2026-08-26",
	}

	for i := range expected {
		if cells[i] != expected[i] {
			t.Fatalf(
				"cell %d (%s) = %q, want %q",
				i,
				header[i],
				cells[i],
				expected[i],
			)
		}
	}
}

// A job with no discovery timestamp must produce an empty cell rather than a
// zero-value date, which would read as a real date from year 1.
func TestRowCellsOmitsZeroDate(t *testing.T) {
	cells := Row{JobID: 1}.cells()

	if cells[len(cells)-1] != "" {
		t.Fatalf(
			"date cell = %q, want empty for a zero timestamp",
			cells[len(cells)-1],
		)
	}
}

func TestNewGogWriterRequiresSpreadsheetID(t *testing.T) {
	if _, err := NewGogWriter(Config{}); err == nil {
		t.Fatal("a missing spreadsheet ID should be refused")
	}

	if _, err := NewGogWriter(
		Config{SpreadsheetID: "   "},
	); err == nil {
		t.Fatal("a blank spreadsheet ID should be refused")
	}
}

func TestNewGogWriterAppliesDefaults(t *testing.T) {
	writer, err := NewGogWriter(Config{SpreadsheetID: "abc123"})
	if err != nil {
		t.Fatal(err)
	}

	if writer.config.SheetName != "Jobs" {
		t.Fatalf("sheet name = %q, want Jobs", writer.config.SheetName)
	}

	if writer.config.Binary != "gog" {
		t.Fatalf("binary = %q, want gog", writer.config.Binary)
	}
}

// Appending nothing must not shell out at all. A misconfigured binary would
// otherwise turn an empty run into a spurious failure on every timer tick.
func TestAppendWithNoRowsDoesNothing(t *testing.T) {
	writer, err := NewGogWriter(Config{
		SpreadsheetID: "abc123",
		Binary:        "/nonexistent/gog",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := writer.Append(nil, nil); err != nil {
		t.Fatalf("appending zero rows should be a no-op, got: %v", err)
	}
}

// The keyring failure is the most likely first-run problem, so the error must
// name the fix rather than surfacing a bare exit status.
func TestSummarizeSurfacesKeyringHint(t *testing.T) {
	output := "WARN read token: no TTY available for keyring file backend " +
		"password prompt; set GOG_KEYRING_PASSWORD"

	summary := summarize(output)

	if !contains(summary, "GOG_KEYRING_PASSWORD") {
		t.Fatalf("summary = %q, want it to name the environment variable", summary)
	}

	if !contains(summary, "keyring") {
		t.Fatalf("summary = %q, want it to mention the keyring", summary)
	}
}

func TestSummarizeHandlesEmptyOutput(t *testing.T) {
	if summarize("   ") != "(no output)" {
		t.Fatalf("summarize of blank output = %q", summarize("   "))
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}

			return false
		}()
}
