package application

import (
	"encoding/json"
	"testing"
)

func TestGreenhouseFormToQuestionnaireInputs(t *testing.T) {
	// Shaped after Stripe's real Backend Engineer form, which is where the
	// artifact-field and multi-select cases come from.
	form := GreenhouseApplicationForm{
		JobID: "6042172",
		Fields: []GreenhouseFormField{
			{
				Name:     "first_name",
				Label:    "First Name",
				Type:     "input_text",
				Required: true,
			},
			{
				Name:     "resume",
				Label:    "Resume/CV",
				Type:     "input_file",
				Required: false,
			},
			{
				Name:     "resume_text",
				Label:    "Resume/CV",
				Type:     "textarea",
				Required: false,
			},
			{
				Name:     "cover_letter_text",
				Label:    "Cover Letter",
				Type:     "textarea",
				Required: false,
			},
			{
				ID:   "48620089",
				Name: "question_48620089",
				// Trailing newline and doubled spacing, as Greenhouse returns.
				Label:    "Please  select the country where you currently reside.\n",
				Type:     "multi_value_single_select",
				Required: true,
				Options: []GreenhouseFormOption{
					{Value: "1", Label: "India"},
					{Value: "2", Label: "United States"},
					{Value: "3", Label: ""},
				},
			},
			{
				ID:       "48620090",
				Name:     "question_48620090[]",
				Label:    "Countries you anticipate working in",
				Type:     "multi_value_multi_select",
				Required: true,
			},
			{
				Name:  "",
				Label: "unnamed field",
				Type:  "input_text",
			},
		},
	}

	inputs := GreenhouseFormToQuestionnaireInputs(form)

	// first_name plus the two selects. The file field, both artifact-backed
	// text fields, and the unnamed field are all dropped.
	if len(inputs) != 3 {
		t.Fatalf(
			"inputs = %d, want 3; got %+v",
			len(inputs),
			inputs,
		)
	}

	if inputs[0].FieldKey != "first_name" {
		t.Fatalf("first field key = %q, want first_name", inputs[0].FieldKey)
	}

	// Whitespace in employer-authored labels must be normalized.
	wantLabel := "Please select the country where you currently reside."

	if inputs[1].Question != wantLabel {
		t.Fatalf(
			"label = %q, want %q",
			inputs[1].Question,
			wantLabel,
		)
	}

	// The [] suffix marks a multi-select in Greenhouse. The stored key drops it
	// so the answer bank can match a stable name.
	if inputs[2].FieldKey != "question_48620090" {
		t.Fatalf(
			"multi-select field key = %q, want question_48620090",
			inputs[2].FieldKey,
		)
	}

	var metadata greenhouseFieldMetadata

	if err := json.Unmarshal(
		[]byte(inputs[2].Metadata),
		&metadata,
	); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}

	// The original name including [] must survive for submission.
	if metadata.FieldName != "question_48620090[]" {
		t.Fatalf(
			"metadata field name = %q, want the original including []",
			metadata.FieldName,
		)
	}

	if !metadata.Required {
		t.Fatal("required flag lost in metadata")
	}

	if metadata.Source != "greenhouse" {
		t.Fatalf("metadata source = %q, want greenhouse", metadata.Source)
	}

	// Select options are needed so an answer can be matched to a valid value.
	var selectMetadata greenhouseFieldMetadata

	if err := json.Unmarshal(
		[]byte(inputs[1].Metadata),
		&selectMetadata,
	); err != nil {
		t.Fatalf("decode select metadata: %v", err)
	}

	if len(selectMetadata.Options) != 2 {
		t.Fatalf(
			"options = %v, want 2 with the blank one dropped",
			selectMetadata.Options,
		)
	}
}

// Artifact-backed fields must never become questions: answer resolution cannot
// satisfy them, so they would block readiness forever.
func TestGreenhouseFormSkipsArtifactFields(t *testing.T) {
	for _, name := range []string{
		"resume",
		"resume_text",
		"cover_letter",
		"cover_letter_text",
	} {
		t.Run(name, func(t *testing.T) {
			inputs := GreenhouseFormToQuestionnaireInputs(
				GreenhouseApplicationForm{
					Fields: []GreenhouseFormField{
						{
							Name:  name,
							Label: "Attachment",
							Type:  "textarea",
						},
					},
				},
			)

			if len(inputs) != 0 {
				t.Fatalf(
					"field %q produced %d input(s), want 0",
					name,
					len(inputs),
				)
			}
		})
	}
}

func TestGreenhouseFormFallsBackToFieldNameWhenLabelMissing(t *testing.T) {
	inputs := GreenhouseFormToQuestionnaireInputs(
		GreenhouseApplicationForm{
			Fields: []GreenhouseFormField{
				{
					Name:  "phone",
					Label: "   ",
					Type:  "input_text",
				},
			},
		},
	)

	if len(inputs) != 1 {
		t.Fatalf("inputs = %d, want 1", len(inputs))
	}

	if inputs[0].Question != "phone" {
		t.Fatalf(
			"question = %q, want the field name as fallback",
			inputs[0].Question,
		)
	}
}
