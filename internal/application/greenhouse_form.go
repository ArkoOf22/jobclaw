package application

import (
	"fmt"
	"strings"
)

type GreenhouseApplicationForm struct {
	JobID  string
	Fields []GreenhouseFormField
}

type GreenhouseFormField struct {
	ID       string
	Name     string
	Label    string
	Type     string
	Required bool
	Options  []GreenhouseFormOption
}

type GreenhouseFormOption struct {
	Value string
	Label string
}

type GreenhouseMappedField struct {
	Name  string
	Value string
}

type GreenhouseFieldMapper struct{}

func NewGreenhouseFieldMapper() *GreenhouseFieldMapper {
	return &GreenhouseFieldMapper{}
}

func (m *GreenhouseFieldMapper) Map(
	form GreenhouseApplicationForm,
	answers []ResolvedSubmissionAnswer,
) ([]GreenhouseMappedField, error) {
	if strings.TrimSpace(form.JobID) == "" {
		return nil, fmt.Errorf("greenhouse form job ID is required")
	}

	answerByKey := make(map[string]ResolvedSubmissionAnswer, len(answers))

	for _, answer := range answers {
		key := normalizeGreenhouseFieldKey(answer.FieldKey)
		if key == "" {
			continue
		}

		answerByKey[key] = answer
	}

	mapped := make(
		[]GreenhouseMappedField,
		0,
		len(form.Fields),
	)

	for _, field := range form.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return nil, fmt.Errorf(
				"greenhouse form contains field with empty name",
			)
		}

		key := normalizeGreenhouseFieldKey(name)

		answer, ok := answerByKey[key]
		if !ok {
			if field.Required {
				return nil, fmt.Errorf(
					"required greenhouse field %q has no answer",
					name,
				)
			}

			continue
		}

		value := strings.TrimSpace(answer.Answer)
		if value == "" {
			if field.Required {
				return nil, fmt.Errorf(
					"required greenhouse field %q has empty answer",
					name,
				)
			}

			continue
		}

		value, err := mapGreenhouseFieldValue(
			field,
			value,
		)
		if err != nil {
			return nil, err
		}

		mapped = append(
			mapped,
			GreenhouseMappedField{
				Name:  name,
				Value: value,
			},
		)
	}

	return mapped, nil
}

func normalizeGreenhouseFieldKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))

	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		".", "_",
	)

	return replacer.Replace(value)
}

func mapGreenhouseFieldValue(
	field GreenhouseFormField,
	answer string,
) (string, error) {
	if len(field.Options) == 0 {
		return answer, nil
	}

	normalizedAnswer := strings.ToLower(
		strings.TrimSpace(answer),
	)

	for _, option := range field.Options {
		if strings.EqualFold(
			strings.TrimSpace(option.Value),
			answer,
		) {
			return option.Value, nil
		}

		if strings.EqualFold(
			strings.TrimSpace(option.Label),
			answer,
		) {
			return option.Value, nil
		}
	}

	for _, option := range field.Options {
		label := strings.ToLower(
			strings.TrimSpace(option.Label),
		)

		value := strings.ToLower(
			strings.TrimSpace(option.Value),
		)

		if normalizedAnswer == label ||
			normalizedAnswer == value {
			return option.Value, nil
		}
	}

	return "", fmt.Errorf(
		"answer %q does not match any option for greenhouse field %q",
		answer,
		field.Name,
	)
}
