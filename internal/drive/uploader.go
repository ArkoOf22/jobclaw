// Package drive uploads files to Google Drive through the gog CLI.
//
// Like the sheet package, it shells out to gog rather than calling the Drive API
// directly: gog is already installed and OAuth-authorised for the candidate's
// account with the drive.file scope, so this needs no GCP project or service
// account. drive.file limits gog to files it created itself, which is exactly the
// resumes uploaded here.
package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Config struct {
	// Account is the Google account gog should act as.
	Account string

	// Binary is the gog executable. Defaults to "gog".
	Binary string

	// ParentFolderID optionally places uploads in a specific Drive folder.
	// Empty uploads to the account's My Drive root.
	ParentFolderID string
}

type Uploader struct {
	config Config
}

func NewUploader(config Config) *Uploader {
	if strings.TrimSpace(config.Binary) == "" {
		config.Binary = "gog"
	}

	return &Uploader{config: config}
}

// Result is what an upload yields: the Drive file ID and a link to open it.
type Result struct {
	FileID string
	Link   string
}

// gogUploadResponse captures the fields we need from gog's JSON output. gog
// returns the created file resource; field names follow the Drive API.
type gogUploadResponse struct {
	ID          string `json:"id"`
	WebViewLink string `json:"webViewLink"`
	Name        string `json:"name"`
}

// Upload sends a local file to Drive under the given name and returns its ID and
// a viewable link.
func (u *Uploader) Upload(
	ctx context.Context,
	localPath string,
	name string,
) (Result, error) {
	if strings.TrimSpace(localPath) == "" {
		return Result{}, fmt.Errorf("local path is required")
	}

	args := []string{
		"drive", "upload", localPath,
		"--mime-type", "application/pdf",
		"--json",
		"--no-input",
	}

	if strings.TrimSpace(name) != "" {
		args = append(args, "--name", name)
	}

	if strings.TrimSpace(u.config.ParentFolderID) != "" {
		args = append(args, "--parent", u.config.ParentFolderID)
	}

	if u.config.Account != "" {
		args = append(args, "--account", u.config.Account)
	}

	command := exec.CommandContext(ctx, u.config.Binary, args...)

	var stdout, stderr bytes.Buffer

	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return Result{}, fmt.Errorf(
			"gog drive upload failed: %w: %s",
			err,
			summarize(stderr.String()),
		)
	}

	return parseUploadResult(stdout.Bytes())
}

// parseUploadResult pulls the file ID and link out of gog's JSON. gog wraps its
// result in an envelope on some commands, so this searches for the resource
// rather than assuming a fixed shape, and falls back to constructing a link from
// the ID when webViewLink is absent.
func parseUploadResult(output []byte) (Result, error) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return Result{}, fmt.Errorf("gog drive upload produced no output")
	}

	// gog wraps the created resource differently across commands: upload nests it
	// under "file", other commands use "result" or the bare object. Try each.
	var direct gogUploadResponse

	if err := json.Unmarshal(trimmed, &direct); err == nil && direct.ID != "" {
		return resultFrom(direct), nil
	}

	var envelope struct {
		File   gogUploadResponse `json:"file"`
		Result gogUploadResponse `json:"result"`
	}

	if err := json.Unmarshal(trimmed, &envelope); err == nil {
		if envelope.File.ID != "" {
			return resultFrom(envelope.File), nil
		}

		if envelope.Result.ID != "" {
			return resultFrom(envelope.Result), nil
		}
	}

	return Result{}, fmt.Errorf(
		"could not find file ID in gog output: %s",
		firstLines(string(trimmed), 3),
	)
}

func resultFrom(r gogUploadResponse) Result {
	link := r.WebViewLink

	if link == "" && r.ID != "" {
		link = "https://drive.google.com/file/d/" + r.ID + "/view"
	}

	return Result{FileID: r.ID, Link: link}
}

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
	lines := strings.Split(strings.TrimSpace(text), "\n")

	if len(lines) > limit {
		lines = lines[:limit]
	}

	return strings.Join(lines, " | ")
}
