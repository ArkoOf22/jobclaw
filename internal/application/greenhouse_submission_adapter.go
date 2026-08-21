package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type GreenhouseSubmissionAdapter struct {
	httpClient *http.Client
	baseURL    string
}

func NewGreenhouseSubmissionAdapter(
	httpClient *http.Client,
	baseURL string,
) *GreenhouseSubmissionAdapter {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &GreenhouseSubmissionAdapter{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

type greenhouseSubmissionRequest struct {
	JobID   string                       `json:"job_id"`
	Resume  string                       `json:"resume"`
	Answers []greenhouseSubmissionAnswer `json:"answers"`
}

type greenhouseSubmissionAnswer struct {
	FieldKey string `json:"field_key"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type greenhouseSubmissionResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (a *GreenhouseSubmissionAdapter) Submit(
	ctx context.Context,
	request SubmissionRequest,
) (SubmissionResult, error) {
	if err := ctx.Err(); err != nil {
		return SubmissionFailed, err
	}

	if request.Target.Type != SubmissionTargetGreenhouse {
		return SubmissionFailed, fmt.Errorf(
			"invalid submission target type %q",
			request.Target.Type,
		)
	}

	if strings.TrimSpace(request.Target.ExternalID) == "" {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse external job ID is required",
		)
	}

	if len(request.Prepared.Resume) == 0 {
		return SubmissionFailed, fmt.Errorf(
			"prepared resume is empty",
		)
	}

	if a.httpClient == nil {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse HTTP client is not configured",
		)
	}

	if strings.TrimSpace(a.baseURL) == "" {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse base URL is not configured",
		)
	}

	payload := greenhouseSubmissionRequest{
		JobID:  request.Target.ExternalID,
		Resume: string(request.Prepared.Resume),
		Answers: make(
			[]greenhouseSubmissionAnswer,
			0,
			len(request.Prepared.Answers),
		),
	}

	for _, answer := range request.Prepared.Answers {
		payload.Answers = append(
			payload.Answers,
			greenhouseSubmissionAnswer{
				FieldKey: answer.FieldKey,
				Question: answer.Question,
				Answer:   answer.Answer,
			},
		)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"encode greenhouse submission: %w",
			err,
		)
	}

	url := a.baseURL + "/submit"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"create greenhouse submission request: %w",
			err,
		)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		// We cannot know whether the remote system accepted the
		// submission before the connection failed.
		return SubmissionAmbiguous, fmt.Errorf(
			"submit to greenhouse: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse returned HTTP %d",
			resp.StatusCode,
		)
	}

	var result greenhouseSubmissionResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SubmissionAmbiguous, fmt.Errorf(
			"decode greenhouse submission response: %w",
			err,
		)
	}

	if !result.Success {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "greenhouse rejected submission"
		}

		return SubmissionFailed, fmt.Errorf("%s", message)
	}

	return SubmissionSucceeded, nil
}
