package application

import (
	"context"
	"testing"
)

func TestLLMQuestionAnswerGeneratorReturnsAnswer(t *testing.T) {
	llm := NewMockResumeLLM("30 days")

	generator := NewLLMQuestionAnswerGenerator(llm)

	answer, err := generator.GenerateAnswer(
		context.Background(),
		ApplicationQuestion{
			ID:       1,
			Question: "What is your notice period?",
		},
		"Candidate has a notice period of 30 days.",
	)
	if err != nil {
		t.Fatal(err)
	}

	if answer != "30 days" {
		t.Fatalf("answer = %q, want %q", answer, "30 days")
	}
}

func TestLLMQuestionAnswerGeneratorRejectsMissingContext(t *testing.T) {
	llm := NewMockResumeLLM("30 days")

	generator := NewLLMQuestionAnswerGenerator(llm)

	_, err := generator.GenerateAnswer(
		context.Background(),
		ApplicationQuestion{
			ID:       1,
			Question: "What is your notice period?",
		},
		"",
	)

	if err == nil {
		t.Fatal("expected missing candidate context error")
	}
}

func TestLLMQuestionAnswerGeneratorSupportsNeedsReview(t *testing.T) {
	llm := NewMockResumeLLM("NEEDS_REVIEW")

	generator := NewLLMQuestionAnswerGenerator(llm)

	answer, err := generator.GenerateAnswer(
		context.Background(),
		ApplicationQuestion{
			ID:       1,
			Question: "What is your preferred programming language?",
		},
		"Candidate profile contains no programming-language preference.",
	)
	if err != nil {
		t.Fatal(err)
	}

	if answer != "NEEDS_REVIEW" {
		t.Fatalf("answer = %q, want NEEDS_REVIEW", answer)
	}
}

func TestValidateQuestionAnswerRejectsCommentary(t *testing.T) {
	err := validateQuestionAnswer(
		"Here is the answer: 30 days",
	)

	if err == nil {
		t.Fatal("expected commentary validation error")
	}
}

func TestValidateQuestionAnswerRejectsCodeFence(t *testing.T) {
	err := validateQuestionAnswer(
		"```text\n30 days\n```",
	)

	if err == nil {
		t.Fatal("expected code fence validation error")
	}
}
