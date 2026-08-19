package application

import (
	"context"
	"fmt"

	"jobclaw/internal/job"
)

type ManualSubmitter struct{}

func NewManualSubmitter() *ManualSubmitter {
	return &ManualSubmitter{}
}

func (s *ManualSubmitter) Submit(
	ctx context.Context,
	app Application,
	j job.Job,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return fmt.Errorf(
		"manual submission required for application %d (job %d)",
		app.ID,
		j.ID,
	)
}
