package application

import (
	"encoding/json"
	"strings"
)

// artifactFieldNames are Greenhouse fields satisfied by generated artifacts
// rather than by questionnaire answers.
//
// Ingesting these as questions would create entries that answer resolution can
// never satisfy, permanently blocking readiness. The resume is attached from
// Application.TailoredResumePath and cover letters are a separate artifact type.
var artifactFieldNames = map[string]bool{
	"resume":            true,
	"resume_text":       true,
	"cover_letter":      true,
	"cover_letter_text": true,
}

// greenhouseFieldMetadata is persisted alongside each question so answer
// resolution and the submission preview can see the field's real shape,
// including the exact option values a select expects.
type greenhouseFieldMetadata struct {
	Source    string   `json:"source"`
	GreenheID string   `json:"greenhouse_field_id,omitempty"`
	FieldName string   `json:"field_name"`
	FieldType string   `json:"field_type"`
	Required  bool     `json:"required"`
	Options   []string `json:"options,omitempty"`
}

// GreenhouseFormToQuestionnaireInputs converts a fetched Greenhouse application
// form into questionnaire inputs ready for ingestion.
//
// This is the bridge that was missing: the HTTP form provider existed and the
// ingestor existed, but nothing connected them, so questions could only be
// ingested from a hand-written local text file.
//
// File-upload fields and artifact-backed fields are skipped, as are fields with
// no usable name.
func GreenhouseFormToQuestionnaireInputs(
	form GreenhouseApplicationForm,
) []QuestionnaireInput {
	inputs := make([]QuestionnaireInput, 0, len(form.Fields))

	for _, field := range form.Fields {
		name := strings.TrimSpace(field.Name)

		if name == "" {
			continue
		}

		// Greenhouse uses a trailing [] to mark multi-select fields. Strip it
		// for the stored field key so the answer bank can match on a stable
		// name, but keep the original in metadata for submission.
		lookupName := strings.TrimSuffix(name, "[]")

		if artifactFieldNames[lookupName] {
			continue
		}

		if field.Type == "input_file" {
			continue
		}

		label := strings.TrimSpace(field.Label)

		if label == "" {
			label = lookupName
		}

		// Greenhouse labels frequently carry trailing newlines and doubled
		// spaces from the employer's form editor.
		label = strings.Join(strings.Fields(label), " ")

		options := make([]string, 0, len(field.Options))

		for _, option := range field.Options {
			optionLabel := strings.TrimSpace(option.Label)

			if optionLabel == "" {
				continue
			}

			options = append(options, optionLabel)
		}

		metadata := greenhouseFieldMetadata{
			Source:    "greenhouse",
			GreenheID: field.ID,
			FieldName: name,
			FieldType: field.Type,
			Required:  field.Required,
			Options:   options,
		}

		encoded, err := json.Marshal(metadata)
		if err != nil {
			// Metadata is descriptive. Losing it must not drop the question,
			// which is the part readiness depends on.
			encoded = []byte("{}")
		}

		inputs = append(inputs, QuestionnaireInput{
			Question: label,
			FieldKey: lookupName,
			Metadata: string(encoded),
		})
	}

	return inputs
}
