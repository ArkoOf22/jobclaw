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

// dataRange covers every data row, leaving the header alone.
//
// Open-ended on purpose: the sheet grows, and a bounded range would silently
// leave rows behind once it outgrew the bound.
func (w *GogWriter) dataRange() string {
	return fmt.Sprintf("%s!A2:L", w.config.SheetName)
}

// Snapshot returns the sheet's current contents verbatim, for backup before a
// destructive operation.
//
// Read as raw bytes rather than parsed: the point is to preserve whatever is
// there, including anything typed in by hand that JobClaw does not model.
func (w *GogWriter) Snapshot(ctx context.Context) ([]byte, error) {
	args := []string{
		"sheets", "get",
		w.config.SpreadsheetID,
		w.config.SheetName,
		"--json",
		"--no-input",
	}

	if w.config.Account != "" {
		args = append(args, "--account", w.config.Account)
	}

	command := exec.CommandContext(ctx, w.config.Binary, args...)

	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return nil, fmt.Errorf(
			"gog sheets get failed: %w: %s",
			err,
			summarize(stderr.String()),
		)
	}

	return stdout.Bytes(), nil
}

// Clear removes every data row, keeping the header.
//
// Used by the rebuild path. Destructive and not reversible from JobClaw's side,
// so callers must take a Snapshot first.
func (w *GogWriter) Clear(ctx context.Context) error {
	args := []string{
		"sheets", "clear",
		w.config.SpreadsheetID,
		w.dataRange(),
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
			"gog sheets clear failed: %w: %s",
			err,
			summarize(stderr.String()),
		)
	}

	return nil
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

// Column letters, derived from Header() order. Kept as constants because the
// Sheets API addresses cells by letter, not by index.
//
// columnStatus (J) can be turned into a click-to-pick dropdown in the Sheet UI.
// It is a one-time manual step because gog cannot send a setDataValidation
// request; see docs/sheet-status-dropdown.md for the setup and the exact value
// list. Note the sheet is one-way: a dropdown edit does not update the database,
// so `jobclaw mark` remains the source-of-truth update.
const (
	columnResume = "I"
	columnStatus = "J"
)

// FindRowByJobID returns the 1-based sheet row for a job, or 0 when absent.
//
// Job IDs live in column A, so only that column is read. The sheet is written in
// score order rather than ID order, and rows are never renumbered, so the row for
// a job cannot be computed and has to be looked up.
func (w *GogWriter) FindRowByJobID(
	ctx context.Context,
	jobID int64,
) (int, error) {
	args := []string{
		"sheets", "get",
		w.config.SpreadsheetID,
		w.config.SheetName + "!A1:A1000",
		"--plain",
		"--no-input",
	}

	if w.config.Account != "" {
		args = append(args, "--account", w.config.Account)
	}

	command := exec.CommandContext(ctx, w.config.Binary, args...)

	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return 0, fmt.Errorf(
			"read sheet job IDs: %w: %s",
			err,
			summarize(stderr.String()),
		)
	}

	want := fmt.Sprintf("%d", jobID)

	for index, line := range strings.Split(
		strings.TrimRight(stdout.String(), "\n"),
		"\n",
	) {
		if strings.TrimSpace(line) == want {
			// Sheets rows are 1-based.
			return index + 1, nil
		}
	}

	return 0, nil
}

// RowUpdate carries the cells to change. Empty fields are left untouched, so a
// status change does not blank an existing resume link.
type RowUpdate struct {
	Status     string
	ResumeLink string
}

// UpdateJobRow changes the status and resume cells for a job.
//
// Returns false when the job has no row yet, which happens for a job acted on
// before the sheet was synced. That is not an error: the caller decides whether
// to care.
func (w *GogWriter) UpdateJobRow(
	ctx context.Context,
	jobID int64,
	update RowUpdate,
) (bool, error) {
	row, err := w.FindRowByJobID(ctx, jobID)
	if err != nil {
		return false, err
	}

	if row == 0 {
		return false, nil
	}

	// Each cell is written separately rather than as one range, because the
	// resume and status columns are not adjacent and a range write would clobber
	// the "Why" column between them.
	if update.ResumeLink != "" {
		if err := w.updateCell(
			ctx,
			fmt.Sprintf("%s%d", columnResume, row),
			update.ResumeLink,
		); err != nil {
			return false, err
		}
	}

	if update.Status != "" {
		if err := w.updateCell(
			ctx,
			fmt.Sprintf("%s%d", columnStatus, row),
			update.Status,
		); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (w *GogWriter) updateCell(
	ctx context.Context,
	cell string,
	value string,
) error {
	encoded, err := json.Marshal([][]string{{value}})
	if err != nil {
		return fmt.Errorf("encode cell value: %w", err)
	}

	args := []string{
		"sheets", "update",
		w.config.SpreadsheetID,
		w.config.SheetName + "!" + cell,
		"--values-json", string(encoded),
		"--input", "USER_ENTERED",
		"--no-input",
	}

	if w.config.Account != "" {
		args = append(args, "--account", w.config.Account)
	}

	if w.config.DryRun {
		args = append(args, "--dry-run")
	}

	command := exec.CommandContext(ctx, w.config.Binary, args...)

	var stderr bytes.Buffer

	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf(
			"update sheet cell %s: %w: %s",
			cell,
			err,
			summarize(stderr.String()),
		)
	}

	return nil
}
