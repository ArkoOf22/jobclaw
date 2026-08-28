package application

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PDFCompiler turns a LaTeX document into a PDF on disk. Behind an interface so
// the workflow does not shell out directly and can be tested with a fake.
type PDFCompiler interface {
	// Compile writes the given LaTeX source into workDir, produces a PDF named
	// outputName, and returns the path to it.
	Compile(
		ctx context.Context,
		latex string,
		workDir string,
		outputName string,
	) (string, error)
}

// PdfLatexCompiler compiles with the pdflatex binary. The template relies on
// pdfTeX features (glyphtounicode), so pdflatex is the correct engine rather
// than a XeTeX-based one.
type PdfLatexCompiler struct {
	// Binary is the pdflatex executable. Empty means "pdflatex" on PATH.
	Binary string
}

func NewPdfLatexCompiler(binary string) *PdfLatexCompiler {
	if strings.TrimSpace(binary) == "" {
		binary = "pdflatex"
	}

	return &PdfLatexCompiler{Binary: binary}
}

func (c *PdfLatexCompiler) Compile(
	ctx context.Context,
	latex string,
	workDir string,
	outputName string,
) (string, error) {
	if strings.TrimSpace(latex) == "" {
		return "", fmt.Errorf("latex source is empty")
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("create resume work dir: %w", err)
	}

	jobName := strings.TrimSuffix(outputName, ".pdf")
	texPath := filepath.Join(workDir, jobName+".tex")

	if err := os.WriteFile(texPath, []byte(latex), 0644); err != nil {
		return "", fmt.Errorf("write latex source: %w", err)
	}

	// Two passes: the tabular* headings and page layout settle their widths on
	// the second run. Booktabs-style resumes render wrong on a single pass.
	for pass := 0; pass < 2; pass++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		cmd := exec.CommandContext(
			ctx,
			c.Binary,
			"-interaction=nonstopmode",
			"-halt-on-error",
			"-output-directory="+workDir,
			texPath,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf(
				"pdflatex failed: %w\n%s",
				err,
				lastLatexErrors(string(output)),
			)
		}
	}

	pdfPath := filepath.Join(workDir, jobName+".pdf")

	info, err := os.Stat(pdfPath)
	if err != nil {
		return "", fmt.Errorf("pdflatex produced no PDF: %w", err)
	}

	if info.Size() == 0 {
		return "", fmt.Errorf("pdflatex produced an empty PDF")
	}

	return pdfPath, nil
}

// lastLatexErrors extracts the lines pdflatex marks with a leading "!", which is
// where its real error messages live, so a failure reports the LaTeX problem
// rather than a wall of log output.
func lastLatexErrors(log string) string {
	var errors []string

	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "!") {
			errors = append(errors, line)
		}
	}

	if len(errors) == 0 {
		// No marked error line; fall back to the tail of the log.
		lines := strings.Split(strings.TrimRight(log, "\n"), "\n")
		if len(lines) > 12 {
			lines = lines[len(lines)-12:]
		}

		return strings.Join(lines, "\n")
	}

	return strings.Join(errors, "\n")
}
