package application

import "context"

type Repository interface {
	Create(ctx context.Context, app Application) error
	GetByJobID(ctx context.Context, jobID int64) (*Application, error)
	UpdateTailoredResumePath(ctx context.Context, id int64, path string) error
	UpdateStatus(ctx context.Context, id int64, status Status) error
}
