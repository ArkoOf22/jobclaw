package application

import (
	"context"
	"fmt"
	"strings"
)

type MockResumeLLM struct {
	Response string
}

func NewMockResumeLLM(response string) *MockResumeLLM {
	return &MockResumeLLM{
		Response: response,
	}
}

func (m *MockResumeLLM) Generate(
	ctx context.Context,
	prompt string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if strings.TrimSpace(m.Response) == "" {
		return "", fmt.Errorf("mock response is empty")
	}

	return m.Response, nil
}
