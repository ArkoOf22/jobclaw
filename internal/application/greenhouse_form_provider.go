package application

import (
	"context"
	"fmt"
	"strings"

	"jobclaw/internal/job"
)

type GreenhouseFormProvider interface {
	GetApplicationForm(
		ctx context.Context,
		j job.Job,
	) (GreenhouseApplicationForm, error)
}

type StaticGreenhouseFormProvider struct {
	forms map[string]GreenhouseApplicationForm
}

func NewStaticGreenhouseFormProvider(
	forms ...GreenhouseApplicationForm,
) (*StaticGreenhouseFormProvider, error) {
	provider := &StaticGreenhouseFormProvider{
		forms: make(map[string]GreenhouseApplicationForm, len(forms)),
	}

	for _, form := range forms {
		jobID := strings.TrimSpace(form.JobID)
		if jobID == "" {
			return nil, fmt.Errorf(
				"greenhouse form job ID is required",
			)
		}

		if _, exists := provider.forms[jobID]; exists {
			return nil, fmt.Errorf(
				"duplicate greenhouse form for job %q",
				jobID,
			)
		}

		provider.forms[jobID] = form
	}

	return provider, nil
}

func (p *StaticGreenhouseFormProvider) GetApplicationForm(
	ctx context.Context,
	j job.Job,
) (GreenhouseApplicationForm, error) {
	jobID := strings.TrimSpace(j.ExternalID)
	if err := ctx.Err(); err != nil {
		return GreenhouseApplicationForm{}, err
	}

	if p == nil {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse form provider is not configured",
		)
	}

	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse form job ID is required",
		)
	}

	form, ok := p.forms[jobID]
	if !ok {
		return GreenhouseApplicationForm{}, fmt.Errorf(
			"greenhouse application form not found for job %q",
			jobID,
		)
	}

	return form, nil
}
