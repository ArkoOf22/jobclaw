package application

import (
	"context"
	"fmt"
	"sync"

	"jobclaw/internal/job"
)

type StaticSubmissionAdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[SubmissionTargetType]SubmissionAdapter
}

func NewStaticSubmissionAdapterRegistry() *StaticSubmissionAdapterRegistry {
	return &StaticSubmissionAdapterRegistry{
		adapters: make(map[SubmissionTargetType]SubmissionAdapter),
	}
}

func (r *StaticSubmissionAdapterRegistry) Register(
	targetType SubmissionTargetType,
	adapter SubmissionAdapter,
) error {
	if targetType == "" {
		return fmt.Errorf("submission target type is required")
	}

	if adapter == nil {
		return fmt.Errorf(
			"submission adapter is required for target type %s",
			targetType,
		)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.adapters[targetType] = adapter
	return nil
}

func (r *StaticSubmissionAdapterRegistry) Get(
	targetType SubmissionTargetType,
) (SubmissionAdapter, error) {
	if targetType == "" {
		return nil, fmt.Errorf("submission target type is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[targetType]
	if !ok {
		return nil, fmt.Errorf(
			"no submission adapter registered for target type %s",
			targetType,
		)
	}

	return adapter, nil
}

type RegistrySubmissionSubmitter struct {
	registry SubmissionAdapterRegistry
}

func NewRegistrySubmissionSubmitter(
	registry SubmissionAdapterRegistry,
) *RegistrySubmissionSubmitter {
	return &RegistrySubmissionSubmitter{
		registry: registry,
	}
}

func (s *RegistrySubmissionSubmitter) Submit(
	ctx context.Context,
	app Application,
	j job.Job,
) (SubmissionResult, error) {
	if s.registry == nil {
		return SubmissionFailed, fmt.Errorf(
			"submission adapter registry is not configured",
		)
	}

	target := ResolveSubmissionTarget(j)

	adapter, err := s.registry.Get(target.Type)
	if err != nil {
		return SubmissionFailed, err
	}

	return adapter.Submit(ctx, SubmissionRequest{
		Application: app,
		Job:         j,
		Target:      target,
	})
}
