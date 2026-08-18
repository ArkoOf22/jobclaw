package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResumeSourceLoadsMasterResume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "master_resume.txt")

	content := `Arkodeep Koley

EXPERIENCE

Twid — Software Development Engineer

- Built Go microservices.
- Worked with Kafka.
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: path,
	})

	got, err := source.Load()
	if err != nil {
		t.Fatal(err)
	}

	if got != content {
		t.Fatalf("loaded resume differs from source:\n%q", got)
	}

	if !strings.Contains(got, "Twid") {
		t.Fatalf("expected resume content, got: %q", got)
	}
}

func TestResumeSourceRejectsMissingPath(t *testing.T) {
	source := NewResumeSource(ResumeSourceConfig{})

	if _, err := source.Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestResumeSourceRejectsMissingFile(t *testing.T) {
	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: filepath.Join(
			t.TempDir(),
			"does-not-exist.txt",
		),
	})

	if _, err := source.Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestResumeSourceRejectsEmptyResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	source := NewResumeSource(ResumeSourceConfig{
		MasterPath: path,
	})

	if _, err := source.Load(); err == nil {
		t.Fatal("expected error")
	}
}
