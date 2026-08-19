package application

import (
	"context"
	"testing"
)

func TestApplicationPreparationMarksReadyApplication(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:                 100,
				JobID:              10,
				Status:             StatusDraft,
				TailoredResumePath: "resume.txt",
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
		},
	}

	events := &fakeEventRepository{}

	service := NewApplicationPreparationService(
		applications,
		questions,
		events,
		NewQuestionnaireService(
			questions,
			NewAnswerResolver(&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{
					"notice_period": {
						FieldKey: "notice_period",
						Answer:   "60 days",
						Verified: true,
					},
				},
			}),
			nil,
			"",
			events,
		),
	)

	result, err := service.Prepare(
		context.Background(),
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Ready() {
		t.Fatalf(
			"readiness = %q, want READY",
			result.Status,
		)
	}

	app, err := applications.GetByID(
		context.Background(),
		100,
	)
	if err != nil {
		t.Fatal(err)
	}

	if app.Status != StatusReadyToApply {
		t.Fatalf(
			"status = %q, want %q",
			app.Status,
			StatusReadyToApply,
		)
	}

	if len(events.events) != 2 {
		t.Fatalf(
			"events = %d, want 2",
			len(events.events),
		)
	}

	if events.events[0].Type != EventQuestionAnswered {
		t.Fatalf(
			"event[0] = %q, want %q",
			events.events[0].Type,
			EventQuestionAnswered,
		)
	}

	if events.events[1].Type != EventApplicationReady {
		t.Fatalf(
			"event[1] = %q, want %q",
			events.events[1].Type,
			EventApplicationReady,
		)
	}
}

func TestApplicationPreparationDoesNotAdvanceBlockedApplication(
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

	questions := &fakeQuestionRepository{}

	events := &fakeEventRepository{}

	service := NewApplicationPreparationService(
		applications,
		questions,
		events,
		NewQuestionnaireService(
			questions,
			NewAnswerResolver(&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{},
			}),
			nil,
			"",
			events,
		),
	)

	result, err := service.Prepare(
		context.Background(),
		101,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.Ready() {
		t.Fatal("expected application to remain blocked")
	}

	app, err := applications.GetByID(
		context.Background(),
		101,
	)
	if err != nil {
		t.Fatal(err)
	}

	if app.Status != StatusDraft {
		t.Fatalf(
			"status = %q, want %q",
			app.Status,
			StatusDraft,
		)
	}

	if len(events.events) != 0 {
		t.Fatalf(
			"events = %d, want 0",
			len(events.events),
		)
	}
}

func TestApplicationPreparationIsIdempotent(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			102: {
				ID:                 102,
				JobID:              12,
				Status:             StatusReadyToApply,
				TailoredResumePath: "resume.txt",
			},
		},
	}

	events := &fakeEventRepository{}

	service := NewApplicationPreparationService(
		applications,
		&fakeQuestionRepository{},
		events,
		NewQuestionnaireService(
			&fakeQuestionRepository{},
			NewAnswerResolver(&fakeAnswerRepository{
				answers: map[string]*CandidateAnswer{},
			}),
			nil,
			"",
			events,
		),
	)

	_, err := service.Prepare(
		context.Background(),
		102,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(events.events) != 0 {
		t.Fatalf(
			"events = %d, want 0 for already-ready application",
			len(events.events),
		)
	}
}
