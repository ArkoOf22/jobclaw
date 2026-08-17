package application

import "context"

type ResumeGenerator interface {
	GenerateTailoredResume(
		ctx context.Context,
		jobID int64,
		outputPath string,
	) error
}
