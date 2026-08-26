// Package sheet writes the job pipeline to a Google Sheet so it can be reviewed
// and worked through from a phone.
//
// Writing goes through the `gog` CLI rather than the Sheets API directly. gog is
// already installed and OAuth-authorised for the candidate's Google account, so
// this avoids standing up a GCP project, a service account, and a second
// long-lived credential on the host purely to append rows.
package sheet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Row is one job as presented in the sheet.
//
// Field order here is the column order in the sheet. Appending a new field is
// safe; reordering is not, because existing rows would no longer line up.
type Row struct {
	JobID          int64
	Score          string
	Recommendation string
	Company        string
	Title          string
	Location       string
	Source         string
	ApplyURL       string
	ResumeLink     string
	Status         string
	Reasoning      string
	DiscoveredAt   time.Time
}

// Header is written once when a sheet is first set up.
func Header() []string {
	return []string{
		"Job ID",
		"Score",
		"Verdict",
		"Company",
		"Title",
		"Location",
		"Source",
		"Apply Link",
		"Resume",
		"Status",
		"Why",
		"Found",
	}
}

func (r Row) cells() []string {
	discovered := ""

	if !r.DiscoveredAt.IsZero() {
		discovered = r.DiscoveredAt.Format("2006-01-02")
	}

	return []string{
		fmt.Sprintf("%d", r.JobID),
		r.Score,
		r.Recommendation,
		r.Company,
		r.Title,
		r.Location,
		r.Source,
		r.ApplyURL,
		r.ResumeLink,
		r.Status,
		r.Reasoning,
		discovered,
	}
}

// Writer appends rows to a destination. Implemented by GogWriter, and by fakes
// in tests so sheet formatting can be verified without network access.
type Writer interface {
	Append(ctx context.Context, rows []Row) error
}

// Config describes where and how to write.
type Config struct {
	// SpreadsheetID is the ID from the sheet's URL.
	SpreadsheetID string

	// SheetName is the tab to append to.
	SheetName string

	// Account is the Google account gog should act as.
	Account string

	// Binary is the gog executable. Defaults to "gog".
	Binary string

	// DryRun asks gog to report what it would do without writing.
	DryRun bool
}

type GogWriter struct {
	config Config
}

func NewGogWriter(config Config) (*GogWriter, error) {
	if strings.TrimSpace(config.SpreadsheetID) == "" {
		return nil, fmt.Errorf("spreadsheet ID is required")
	}

	if strings.TrimSpace(config.SheetName) == "" {
		config.SheetName = "Jobs"
	}

	if strings.TrimSpace(config.Binary) == "" {
		config.Binary = "gog"
	}

	return &GogWriter{config: config}, nil
}

// Append adds rows to the bottom of the sheet.
func (w *GogWriter) Append(ctx context.Context, rows []Row) error {
	if len(rows) == 0 {
		return nil
	}

	values := make([][]string, 0, len(rows))

	for _, row := range rows {
		values = append(values, row.cells())
	}

	return w.appendValues(ctx, values)
}

// AppendHeader writes the column titles. Used when initialising a new sheet.
func (w *GogWriter) AppendHeader(ctx context.Context) error {
	return w.appendValues(ctx, [][]string{Header()})
}

func (w *GogWriter) appendValues(
	ctx context.Context,
	values [][]string,
) error {
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode sheet values: %w", err)
	}

	args := []string{
		"sheets", "append",
		w.config.SpreadsheetID,
		// Anchoring at A1 lets Sheets find the first empty row itself.
		w.config.SheetName + "!A1",
		"--values-json", string(encoded),
		// USER_ENTERED so URLs become clickable links rather than plain text.
		"--input", "USER_ENTERED",
		"--insert", "INSERT_ROWS",
		// Never block waiting on a prompt: this runs from a timer.
		"--no-input",
	}

	if w.config.Account != "" {
		args = append(args, "--account", w.config.Account)
	}

	if w.config.DryRun {
		args = append(args, "--dry-run")
	}

	command := exec.CommandContext(ctx, w.config.Binary, args...)

	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf(
			"gog sheets append failed: %w: %s",
			err,
			summarize(stderr.String()),
		)
	}

	return nil
}

// summarize trims CLI output to something loggable, and surfaces the keyring
// hint explicitly because it is the most likely first-run failure: gog stores its
// OAuth token in a file-backed keyring that needs GOG_KEYRING_PASSWORD when there
// is no terminal to prompt on.
func summarize(output string) string {
	output = strings.TrimSpace(output)

	if output == "" {
		return "(no output)"
	}

	if strings.Contains(output, "GOG_KEYRING_PASSWORD") ||
		strings.Contains(output, "keyring") {
		return "gog could not unlock its credential keyring; " +
			"set GOG_KEYRING_PASSWORD in the environment. Original: " +
			firstLines(output, 2)
	}

	return firstLines(output, 4)
}

func firstLines(text string, limit int) string {
	lines := strings.Split(text, "\n")

	if len(lines) > limit {
		lines = lines[:limit]
	}

	return strings.Join(lines, " | ")
}
