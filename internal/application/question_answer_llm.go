package application

import (
	"context"
	"fmt"
	"strings"
)

type QuestionAnswerLLM interface {
	GenerateAnswer(
		ctx context.Context,
		question ApplicationQuestion,
		candidateContext string,
	) (string, error)
}

type LLMQuestionAnswerGenerator struct {
	llm ResumeLLM
}

func NewLLMQuestionAnswerGenerator(
	llm ResumeLLM,
) *LLMQuestionAnswerGenerator {
	return &LLMQuestionAnswerGenerator{
		llm: llm,
	}
}

func (g *LLMQuestionAnswerGenerator) GenerateAnswer(
	ctx context.Context,
	question ApplicationQuestion,
	candidateContext string,
) (string, error) {
	if question.ID <= 0 {
		return "", fmt.Errorf("question ID must be positive")
	}

	if strings.TrimSpace(question.Question) == "" {
		return "", fmt.Errorf("question is required")
	}

	if g.llm == nil {
		return "", fmt.Errorf("question answer LLM is required")
	}

	if strings.TrimSpace(candidateContext) == "" {
		return "", fmt.Errorf("candidate context is required")
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	prompt := buildQuestionAnswerPrompt(
		question,
		candidateContext,
	)

	answer, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate question answer: %w", err)
	}

	answer = strings.TrimSpace(answer)

	if err := validateQuestionAnswer(answer); err != nil {
		return "", err
	}

	return answer, nil
}

func buildQuestionAnswerPrompt(
	question ApplicationQuestion,
	candidateContext string,
) string {
	return fmt.Sprintf(`You are helping prepare a job application.

Candidate context:
%s

Application question:
%s

Answer the application question using ONLY information explicitly
supported by the candidate context.

Rules:
- Never invent facts.
- Never guess.
- Never infer a number that is not explicitly supported.
- Never invent dates, employers, technologies, salary, authorization,
  visa status, relocation preference, or experience.
- If the candidate context does not contain enough information to answer
  safely, return exactly: NEEDS_REVIEW
- Return only the answer.
- Do not explain your reasoning.
- Do not include markdown.
- Do not include phrases such as "Here is the answer".

Question:
%s
`,
		candidateContext,
		question.Question,
		question.Question,
	)
}

func validateQuestionAnswer(answer string) error {
	answer = strings.TrimSpace(answer)

	if answer == "" {
		return fmt.Errorf("question answer is empty")
	}

	if strings.EqualFold(answer, "NEEDS_REVIEW") {
		return nil
	}

	if strings.Contains(answer, "```") {
		return fmt.Errorf("question answer contains markdown code fence")
	}

	lower := strings.ToLower(answer)

	for _, phrase := range []string{
		"here is the answer",
		"here's the answer",
		"as an ai",
		"i cannot",
		"i can't",
	} {
		if strings.Contains(lower, phrase) {
			return fmt.Errorf(
				"question answer contains unsupported commentary",
			)
		}
	}

	return nil
}
