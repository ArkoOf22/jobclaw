package application

import (
	"context"
	"fmt"

	"jobclaw/internal/job"
)

type SubmissionTransaction interface {
	UpdateApplicationStatus(
		ctx context.Context,
		id int64,
		status Status,
	) error

	UpdateJobStatus(
		ctx context.Context,
		id int64,
		status job.Status,
	) error

	CreateEvent(
		ctx context.Context,
		event Event,
	) error

	Commit() error
	Rollback() error
}

type SubmissionTransactionFactory interface {
	BeginSubmissionTransaction(
		ctx context.Context,
	) (SubmissionTransaction, error)
}

type transactionalApplicationRepository struct {
	tx SubmissionTransaction
}

func (r *transactionalApplicationRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status Status,
) error {
	return r.tx.UpdateApplicationStatus(ctx, id, status)
}

type transactionalJobRepository struct {
	tx SubmissionTransaction
}

func (r *transactionalJobRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status job.Status,
) error {
	return r.tx.UpdateJobStatus(ctx, id, status)
}

type transactionalEventRepository struct {
	tx SubmissionTransaction
}

func (r *transactionalEventRepository) Create(
	ctx context.Context,
	event Event,
) error {
	return r.tx.CreateEvent(ctx, event)
}

func beginSubmissionTransaction(
	ctx context.Context,
	factory SubmissionTransactionFactory,
) (SubmissionTransaction, error) {
	if factory == nil {
		return nil, fmt.Errorf(
			"submission transaction factory is not configured",
		)
	}

	tx, err := factory.BeginSubmissionTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"begin submission transaction: %w",
			err,
		)
	}

	if tx == nil {
		return nil, fmt.Errorf(
			"submission transaction is nil",
		)
	}

	return tx, nil
}
