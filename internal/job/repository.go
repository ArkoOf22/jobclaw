package job

import (
	"context"
)

type Repository interface {
	Upsert(ctx context.Context, j Job) error
	GetBySourceExternalID(ctx context.Context, source, externalID string) (*Job, error)
	GetByID(ctx context.Context, id int64) (*Job, error)
	List(ctx context.Context, limit int) ([]Job, error)
	UpdateStatus(ctx context.Context, id int64, status Status) error
}
