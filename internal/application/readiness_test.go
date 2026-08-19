package application

import (
	"context"
	"testing"
)

type fakeApplicationRepository struct {
	applications map[int64]*Application
}

func (f *fakeApplicationRepository) Create(
	ctx context.Context,
	app Application,
) error {
	if f.applications == nil {
		f.applications = make(map[int64]*Application)
	}

	copy := app

	if copy.ID == 0 {
		copy.ID = int64(len(f.applications) + 1)
	}

	f.applications[copy.ID] = &copy

	return nil
}

func (f *fakeApplicationRepository) GetByID(
	ctx context.Context,
	id int64,
) (*Application, error) {
	app := f.applications[id]

	if app == nil {
		return nil, nil
	}

	copy := *app

	return &copy, nil
}

func (f *fakeApplicationRepository) GetByJobID(
	ctx context.Context,
	jobID int64,
) (*Application, error) {
	for _, app := range f.applications {
		if app.JobID == jobID {
			copy := *app
			return &copy, nil
		}
	}

	return nil, nil
}

func (f *fakeApplicationRepository) UpdateTailoredResumePath(
	ctx context.Context,
	id int64,
	path string,
) error {
	app := f.applications[id]

	if app != nil {
		app.TailoredResumePath = path
	}

	return nil
}

func (f *fakeApplicationRepository) UpdateStatus(
	ctx context.Context,
	id int64,
	status Status,
) error {
	app := f.applications[id]

	if app != nil {
		app.Status = status
	}

	return nil
}

func TestApplicationReadinessIsReadyWhenArtifactsAndQuestionsAreComplete(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:                 100,
				JobID:              10,
				Status:             StatusDraft,
				TailoredResumePath: "data/applications/100/resume/tailored.txt",
			},
		},
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            1,
				ApplicationID: 100,
				Question:      "What is your notice period?",
				Status:        QuestionAnswered,
				Answer:        "60 days",
			},
			{
				ID:            2,
				ApplicationID: 100,
				Question:      "Are you willing to relocate?",
				Status:        QuestionApproved,
				Answer:        "Yes",
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		questions,
	)

	result, err := evaluator.Evaluate(
		context.Background(),
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Ready() {
		t.Fatalf(
			"readiness = %q, want %q; blockers = %v",
			result.Status,
			ReadinessReady,
			result.Blockers,
		)
	}

	if len(result.Blockers) != 0 {
		t.Fatalf(
			"blockers = %v, want none",
			result.Blockers,
		)
	}
}

func TestApplicationReadinessBlocksMissingResume(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			101: {
				ID:     101,
				JobID:  11,
				Status: StatusDraft,
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		&fakeQuestionRepository{},
	)

	result, err := evaluator.Evaluate(
		context.Background(),
		101,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Ready() {
		t.Fatal("expected readiness to be blocked")
	}

	if len(result.Blockers) != 1 {
		t.Fatalf(
			"blockers = %v, want 1 blocker",
			result.Blockers,
		)
	}

	if result.Blockers[0] != "tailored resume is missing" {
		t.Fatalf(
			"blocker = %q, want tailored resume is missing",
			result.Blockers[0],
		)
	}
}

func TestApplicationReadinessBlocksNeedsReviewQuestion(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			103: {
				ID:                 103,
				JobID:              13,
				Status:             StatusDraft,
				TailoredResumePath: "resume.txt",
			},
		},
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            3,
				ApplicationID: 103,
				Question:      "Why do you want to work here?",
				Status:        QuestionNeedsReview,
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		questions,
	)

	result, err := evaluator.Evaluate(
		context.Background(),
		103,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Ready() {
		t.Fatal("expected readiness to be blocked")
	}

	if len(result.Blockers) != 1 {
		t.Fatalf(
			"blockers = %v, want 1 blocker",
			result.Blockers,
		)
	}

	if result.Blockers[0] !=
		"question 3 needs review: Why do you want to work here?" {
		t.Fatalf(
			"blocker = %q",
			result.Blockers[0],
		)
	}
}

func TestApplicationReadinessAllowsNoQuestionnaire(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			104: {
				ID:                 104,
				JobID:              14,
				Status:             StatusDraft,
				TailoredResumePath: "resume.txt",
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		&fakeQuestionRepository{},
	)

	result, err := evaluator.Evaluate(
		context.Background(),
		104,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Ready() {
		t.Fatalf(
			"readiness = %q, want %q",
			result.Status,
			ReadinessReady,
		)
	}
}

func TestApplicationReadinessRejectsMissingApplication(
	t *testing.T,
) {
	evaluator := NewApplicationReadinessEvaluator(
		&fakeApplicationRepository{
			applications: map[int64]*Application{},
		},
		&fakeQuestionRepository{},
	)

	_, err := evaluator.Evaluate(
		context.Background(),
		999,
	)

	if err == nil {
		t.Fatal("expected missing application error")
	}
}

func TestApplicationReadinessHonorsCancellation(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	evaluator := NewApplicationReadinessEvaluator(
		&fakeApplicationRepository{},
		&fakeQuestionRepository{},
	)

	_, err := evaluator.Evaluate(
		ctx,
		100,
	)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
