package application

import "context"

type ResumeLLM interface {
	Generate(
		ctx context.Context,
		prompt string,
	) (string, error)
}
