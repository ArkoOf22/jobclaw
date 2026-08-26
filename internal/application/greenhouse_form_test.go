package application

import (
	"strings"
	"testing"
)

func TestGreenhouseFieldMapperMapsMatchingAnswers(
	t *testing.T,
) {
	mapper := NewGreenhouseFieldMapper()

	fields, err := mapper.Map(
		GreenhouseApplicationForm{
			JobID: "gh-123",
			Fields: []GreenhouseFormField{
				{
					Name:     "work_authorization",
					Label:    "Are you authorized to work?",
					Required: true,
				},
				{
					Name:     "sponsorship_required",
					Label:    "Will you require sponsorship?",
					Required: true,
				},
			},
		},
		[]ResolvedSubmissionAnswer{
			{
				FieldKey: "work_authorization",
				Answer:   "Yes",
			},
			{
				FieldKey: "sponsorship-required",
				Answer:   "No",
			},
		},
	)
	if err != nil {
		t.Fatalf("map: %v", err)
	}

	if len(fields) != 2 {
		t.Fatalf(
			"mapped fields = %d, want 2",
			len(fields),
		)
	}

	if fields[0].Name != "work_authorization" ||
		fields[0].Value != "Yes" {
		t.Fatalf(
			"first field = %+v, want work_authorization=Yes",
			fields[0],
		)
	}

	if fields[1].Name != "sponsorship_required" ||
		fields[1].Value != "No" {
		t.Fatalf(
			"second field = %+v, want sponsorship_required=No",
			fields[1],
		)
	}
}

func TestGreenhouseFieldMapperRejectsMissingRequiredAnswer(
	t *testing.T,
) {
	mapper := NewGreenhouseFieldMapper()

	_, err := mapper.Map(
		GreenhouseApplicationForm{
			JobID: "gh-123",
			Fields: []GreenhouseFormField{
				{
					Name:     "work_authorization",
					Required: true,
				},
			},
		},
		nil,
	)

	if err == nil {
		t.Fatal("expected missing required answer error")
	}

	if !strings.Contains(
		err.Error(),
		"required greenhouse field",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGreenhouseFieldMapperSkipsMissingOptionalAnswer(
	t *testing.T,
) {
	mapper := NewGreenhouseFieldMapper()

	fields, err := mapper.Map(
		GreenhouseApplicationForm{
			JobID: "gh-123",
			Fields: []GreenhouseFormField{
				{
					Name:     "linkedin",
					Required: false,
				},
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("map: %v", err)
	}

	if len(fields) != 0 {
		t.Fatalf(
			"mapped fields = %d, want 0",
			len(fields),
		)
	}
}

func TestGreenhouseFieldMapperMapsOptionLabelToValue(
	t *testing.T,
) {
	mapper := NewGreenhouseFieldMapper()

	fields, err := mapper.Map(
		GreenhouseApplicationForm{
			JobID: "gh-123",
			Fields: []GreenhouseFormField{
				{
					Name:     "work_authorization",
					Required: true,
					Options: []GreenhouseFormOption{
						{
							Value: "yes",
							Label: "Yes, I am authorized",
						},
						{
							Value: "no",
							Label: "No",
						},
					},
				},
			},
		},
		[]ResolvedSubmissionAnswer{
			{
				FieldKey: "work_authorization",
				Answer:   "Yes, I am authorized",
			},
		},
	)
	if err != nil {
		t.Fatalf("map: %v", err)
	}

	if len(fields) != 1 {
		t.Fatalf(
			"mapped fields = %d, want 1",
			len(fields),
		)
	}

	if fields[0].Value != "yes" {
		t.Fatalf(
			"mapped value = %q, want %q",
			fields[0].Value,
			"yes",
		)
	}
}

func TestGreenhouseFieldMapperRejectsInvalidOption(
	t *testing.T,
) {
	mapper := NewGreenhouseFieldMapper()

	_, err := mapper.Map(
		GreenhouseApplicationForm{
			JobID: "gh-123",
			Fields: []GreenhouseFormField{
				{
					Name:     "work_authorization",
					Required: true,
					Options: []GreenhouseFormOption{
						{
							Value: "yes",
							Label: "Yes",
						},
						{
							Value: "no",
							Label: "No",
						},
					},
				},
			},
		},
		[]ResolvedSubmissionAnswer{
			{
				FieldKey: "work_authorization",
				Answer:   "Maybe",
			},
		},
	)

	if err == nil {
		t.Fatal("expected invalid option error")
	}

	if !strings.Contains(
		err.Error(),
		"does not match any option",
	) {
		t.Fatalf("unexpected error: %v", err)
	}
}
