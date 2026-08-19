package application

import (
	"context"
	"testing"
)

func TestTextQuestionnaireExtractorExtractsQuestions(t *testing.T) {
	extractor := NewTextQuestionnaireExtractor()

	inputs, err := extractor.Extract(
		context.Background(),
		`
			What is your notice period?
			Are you legally authorized to work in India?
			Do you require visa sponsorship?
		`,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(inputs) != 3 {
		t.Fatalf("inputs = %d, want 3", len(inputs))
	}

	want := []string{
		"What is your notice period?",
		"Are you legally authorized to work in India?",
		"Do you require visa sponsorship?",
	}

	for i := range want {
		if inputs[i].Question != want[i] {
			t.Fatalf(
				"question[%d] = %q, want %q",
				i,
				inputs[i].Question,
				want[i],
			)
		}
	}
}

func TestTextQuestionnaireExtractorHandlesListPrefixes(t *testing.T) {
	extractor := NewTextQuestionnaireExtractor()

	inputs, err := extractor.Extract(
		context.Background(),
		`
			1. What is your notice period?
			2) Are you willing to relocate?
			- Do you require visa sponsorship?
			* What are your salary expectations?
		`,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(inputs) != 4 {
		t.Fatalf("inputs = %d, want 4", len(inputs))
	}

	want := []string{
		"What is your notice period?",
		"Are you willing to relocate?",
		"Do you require visa sponsorship?",
		"What are your salary expectations?",
	}

	for i := range want {
		if inputs[i].Question != want[i] {
			t.Fatalf(
				"question[%d] = %q, want %q",
				i,
				inputs[i].Question,
				want[i],
			)
		}
	}
}

func TestTextQuestionnaireExtractorRejectsEmptySource(t *testing.T) {
	extractor := NewTextQuestionnaireExtractor()

	_, err := extractor.Extract(
		context.Background(),
		"   \n\n",
	)
	if err == nil {
		t.Fatal("expected empty source error")
	}
}

func TestTextQuestionnaireExtractorHonorsContextCancellation(t *testing.T) {
	extractor := NewTextQuestionnaireExtractor()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := extractor.Extract(
		ctx,
		"What is your notice period?",
	)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
