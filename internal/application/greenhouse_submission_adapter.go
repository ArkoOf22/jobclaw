package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GreenhouseSubmissionAdapter struct {
	httpClient   *http.Client
	baseURL      string
	apiKey       string
	formProvider GreenhouseFormProvider
	fieldMapper  *GreenhouseFieldMapper
}

func NewGreenhouseSubmissionAdapter(
	httpClient *http.Client,
	baseURL string,
) *GreenhouseSubmissionAdapter {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &GreenhouseSubmissionAdapter{
		httpClient:  httpClient,
		baseURL:     strings.TrimRight(baseURL, "/"),
		fieldMapper: NewGreenhouseFieldMapper(),
	}
}

func (a *GreenhouseSubmissionAdapter) SetAPIKey(
	apiKey string,
) {
	a.apiKey = strings.TrimSpace(apiKey)
}

func (a *GreenhouseSubmissionAdapter) SetFormProvider(
	provider GreenhouseFormProvider,
) {
	a.formProvider = provider
}

type greenhouseSubmissionResponse struct {
	ID string `json:"id"`
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

	jobID := strings.TrimSpace(request.Target.ExternalID)
	if jobID == "" {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse external job ID is required",
		)
	}

	if len(request.Prepared.Resume) == 0 {
		return SubmissionFailed, fmt.Errorf(
			"prepared resume is empty",
		)
	}

	if a == nil {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse submission adapter is not configured",
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

	if strings.TrimSpace(a.apiKey) == "" {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse API key is not configured",
		)
	}

	if a.formProvider == nil {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse form provider is not configured",
		)
	}

	if a.fieldMapper == nil {
		return SubmissionFailed, fmt.Errorf(
			"greenhouse field mapper is not configured",
		)
	}

	boardToken, err := parseGreenhouseBoardToken(
		request.Job.URL,
	)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"resolve greenhouse board token: %w",
			err,
		)
	}

	form, err := a.formProvider.GetApplicationForm(
		ctx,
		request.Job,
	)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"get greenhouse application form: %w",
			err,
		)
	}

	fields, err := a.fieldMapper.Map(
		form,
		request.Prepared.Answers,
	)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"map greenhouse application fields: %w",
			err,
		)
	}

	body, contentType, err := buildGreenhouseMultipartPayload(
		request.Prepared.Resume,
		fields,
	)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"build greenhouse submission payload: %w",
			err,
		)
	}

	requestURL, err := url.Parse(fmt.Sprintf(
		"%s/boards/%s/jobs/%s",
		a.baseURL,
		boardToken,
		jobID,
	))
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"build greenhouse submission URL: %w",
			err,
		)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL.String(),
		body,
	)
	if err != nil {
		return SubmissionFailed, fmt.Errorf(
			"create greenhouse submission request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		contentType,
	)

	req.SetBasicAuth(
		a.apiKey,
		"",
	)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return SubmissionAmbiguous, fmt.Errorf(
			"submit to greenhouse: %w",
			err,
		)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return SubmissionAmbiguous, fmt.Errorf(
			"read greenhouse submission response: %w",
			readErr,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(
			string(responseBody),
		)

		if message == "" {
			message = fmt.Sprintf(
				"greenhouse returned HTTP %d",
				resp.StatusCode,
			)
		}

		return SubmissionFailed, fmt.Errorf(
			"%s",
			message,
		)
	}

	if len(bytes.TrimSpace(responseBody)) == 0 {
		return SubmissionSucceeded, nil
	}

	var result greenhouseSubmissionResponse

	if err := json.Unmarshal(
		responseBody,
		&result,
	); err != nil {
		return SubmissionAmbiguous, fmt.Errorf(
			"decode greenhouse submission response: %w",
			err,
		)
	}

	return SubmissionSucceeded, nil
}

func buildGreenhouseMultipartPayload(
	resume []byte,
	fields []GreenhouseMappedField,
) (*bytes.Buffer, string, error) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	resumePart, err := writer.CreateFormFile(
		"resume",
		"resume.pdf",
	)
	if err != nil {
		return nil, "", err
	}

	if _, err := resumePart.Write(resume); err != nil {
		return nil, "", err
	}

	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return nil, "", fmt.Errorf(
				"greenhouse field name is empty",
			)
		}

		if err := writer.WriteField(
			name,
			field.Value,
		); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return &body, writer.FormDataContentType(), nil
}
