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
				ID:     100,
				JobID:  10,
				Status: StatusDraft,
				// Readiness verifies the artifact exists on disk, so this must
				// be a real file rather than a placeholder path.
				TailoredResumePath: writeReadinessArtifact(t, "resume.txt"),
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

	// The question already carried an answer, so resolution skips it and emits
	// no QUESTION_ANSWERED event. Re-announcing an existing answer on every
	// prepare would be noise, and re-resolving it used to wipe it.
	if len(events.events) != 1 {
		t.Fatalf(
			"events = %d, want 1; got %+v",
			len(events.events),
			events.events,
		)
	}

	if events.events[0].Type != EventApplicationReady {
		t.Fatalf(
			"event[0] = %q, want %q",
			events.events[0].Type,
			EventApplicationReady,
		)
	}
}

// Preparation must not overwrite an existing answer. Re-resolving with only the
// verified answer bank downgrades anything it cannot cover to NEEDS_REVIEW,
// which previously erased every answer a questionnaire run had produced.
func TestApplicationPreparationPreservesExistingAnswers(t *testing.T) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:                 100,
				JobID:              10,
				Status:             StatusDraft,
				TailoredResumePath: writeReadinessArtifact(t, "resume.txt"),
			},
		},
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            1,
				ApplicationID: 100,
				Question:      "Why this company?",
				Status:        QuestionAnswered,
				Answer:        "Generated rationale",
				AnswerSource:  AnswerSourceLLM,
			},
		},
	}

	events := &fakeEventRepository{}

	// An empty answer bank: the resolver cannot cover this question at all.
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

	if _, err := service.Prepare(
		context.Background(),
		100,
	); err != nil {
		t.Fatal(err)
	}

	stored := questions.questions[0]

	if stored.Answer != "Generated rationale" {
		t.Fatalf(
			"answer = %q, want it preserved",
			stored.Answer,
		)
	}

	if stored.Status != QuestionAnswered {
		t.Fatalf(
			"status = %q, want %q",
			stored.Status,
			QuestionAnswered,
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
