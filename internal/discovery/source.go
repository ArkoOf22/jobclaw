package discovery

import (
	"context"

	"jobclaw/internal/job"
)

type Source interface {
	Name() string

	Discover(ctx context.Context, request Request) ([]job.Job, error)
}
