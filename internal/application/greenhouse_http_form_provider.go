package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jobclaw/internal/job"
)

type GreenhouseHTTPFormProvider struct {
	httpClient *http.Client
	baseURL    string
}

func NewGreenhouseHTTPFormProvider(
	httpClient *http.Client,
	baseURL string,
) *GreenhouseHTTPFormProvider {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 20 * time.Second,
		}
	}

	return &GreenhouseHTTPFormProvider{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

type greenhouseHTTPJobResponse struct {
	ID        int                      `json:"id"`
	Questions []greenhouseHTTPQuestion `json:"questions"`
}

type greenhouseHTTPQuestion struct {
	Label    string                `json:"label"`
	Required bool                  `json:"required"`
	Fields   []greenhouseHTTPField `json:"fields"`
}

type greenhouseHTTPField struct {
	ID     string                     `json:"id"`
	Name   string                     `json:"name"`
	Type   string                     `json:"type"`
	Values []greenhouseHTTPFieldValue `json:"values"`
}

type greenhouseHTTPFieldValue struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

func (p *GreenhouseHTTPFormProvider) GetApplicationForm(
	ctx context.Context,
	j job.Job,
) (GreenhouseApplicationForm, error) {
	if err := ctx.Err(); err != nil {
		return GreenhouseApplicationForm{}, err
	}

	if p == nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse HTTP form provider is not configured",
		)
	}

	if p.httpClient == nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse form HTTP client is not configured",
		)
	}

	if strings.TrimSpace(p.baseURL) == "" {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse form base URL is not configured",
		)
	}

	jobID := strings.TrimSpace(j.ExternalID)
	if jobID == "" {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse external job ID is required",
		)
	}

	boardToken, err := parseGreenhouseBoardToken(j.URL)
	if err != nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"resolve greenhouse board token: %w",
			err,
		)
	}

	requestURL, err := url.Parse(fmt.Sprintf(
		"%s/boards/%s/jobs/%s",
		p.baseURL,
		boardToken,
		jobID,
	))
	if err != nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"build greenhouse job request URL: %w",
			err,
		)
	}

	query := requestURL.Query()
	query.Set("questions", "true")
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL.String(),
		nil,
	)
	if err != nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"create greenhouse job request: %w",
			err,
		)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"request greenhouse job questions: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse job questions returned HTTP %d",
			resp.StatusCode,
		)
	}

	var payload greenhouseHTTPJobResponse

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"decode greenhouse job questions: %w",
			err,
		)
	}

	form := GreenhouseApplicationForm{
		JobID:  jobID,
		Fields: make([]GreenhouseFormField, 0),
	}

	for _, question := range payload.Questions {
		for _, field := range question.Fields {
			if field.Name == "" {
				return GreenhouseApplicationForm{}, fmt.Errorf(
					"greenhouse question %q contains field with empty name",
					question.Label,
				)
			}

			mapped := GreenhouseFormField{
				ID:       field.ID,
				Name:     field.Name,
				Label:    question.Label,
				Type:     field.Type,
				Required: question.Required,
				Options: make(
					[]GreenhouseFormOption,
					0,
					len(field.Values),
				),
			}

			for _, value := range field.Values {
				mapped.Options = append(
					mapped.Options,
					GreenhouseFormOption{
						Value: fmt.Sprint(value.Value),
						Label: value.Label,
					},
				)
			}

			form.Fields = append(form.Fields, mapped)
		}
	}

	return form, nil
}
