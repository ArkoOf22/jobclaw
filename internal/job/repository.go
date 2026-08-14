package job

import (
	"context"
)

type Repository interface {
	Upsert(ctx context.Context, j Job) error
	GetBySourceExternalID(ctx context.Context, source, externalID string) (*Job, error)
	List(ctx context.Context, limit int) ([]Job, error)
}
