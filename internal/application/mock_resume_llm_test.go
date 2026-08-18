package application

import (
	"context"
	"testing"
)

func TestMockResumeLLMReturnsResponse(t *testing.T) {
	llm := NewMockResumeLLM("tailored resume")

	result, err := llm.Generate(
		context.Background(),
		"tailor this resume",
	)
	if err != nil {
		t.Fatal(err)
	}

	if result != "tailored resume" {
		t.Fatalf("result = %q, want %q", result, "tailored resume")
	}
}

func TestMockResumeLLMRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		response string
	}{
		{
			name:     "empty prompt",
			prompt:   "",
			response: "resume",
		},
		{
			name:     "empty response",
			prompt:   "prompt",
			response: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			llm := NewMockResumeLLM(tt.response)

			if _, err := llm.Generate(
				context.Background(),
				tt.prompt,
			); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMockResumeLLMHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	llm := NewMockResumeLLM("resume")

	if _, err := llm.Generate(ctx, "prompt"); err == nil {
		t.Fatal("expected context cancellation error")
	}
}
