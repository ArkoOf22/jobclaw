package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	resumePath := writeReadinessArtifact(t, "tailored.txt")

	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			100: {
				ID:                 100,
				JobID:              10,
				Status:             StatusDraft,
				TailoredResumePath: resumePath,
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
	).WithEventRepository(ingestedQuestionnaireEvents(100))

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

	// Ingestion is reported so the only blocker is the missing resume.
	evaluator := NewApplicationReadinessEvaluator(
		applications,
		&fakeQuestionRepository{},
	).WithEventRepository(ingestedQuestionnaireEvents(101))

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
				TailoredResumePath: writeReadinessArtifact(t, "resume.txt"),
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
	).WithEventRepository(ingestedQuestionnaireEvents(103))

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

// A form with genuinely no questions is ready, but only because ingestion ran
// and confirmed it. See TestApplicationReadinessBlocksWhenQuestionnaireNeverIngested
// for the case where it did not.
func TestApplicationReadinessAllowsIngestedEmptyQuestionnaire(
	t *testing.T,
) {
	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			104: {
				ID:                 104,
				JobID:              14,
				Status:             StatusDraft,
				TailoredResumePath: writeReadinessArtifact(t, "resume.txt"),
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		&fakeQuestionRepository{},
	).WithEventRepository(ingestedQuestionnaireEvents(104))

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

// writeReadinessArtifact creates a real non-empty file. Readiness verifies that
// artifacts exist on disk, so tests cannot use placeholder path strings.
func writeReadinessArtifact(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)

	if err := os.WriteFile(
		path,
		[]byte("tailored resume contents"),
		0600,
	); err != nil {
		t.Fatalf("write test artifact: %v", err)
	}

	return path
}

// ingestedQuestionnaireEvents reports that questionnaire ingestion ran, which is
// what lets readiness treat zero questions as genuine rather than unverified.
func ingestedQuestionnaireEvents(applicationID int64) *fakeEventRepository {
	return &fakeEventRepository{
		events: []Event{
			{
				ApplicationID: applicationID,
				Type:          EventQuestionnaireIngested,
			},
		},
	}
}

// Zero questions must not be read as "nothing to answer" unless ingestion
// actually ran. Otherwise an application whose form was never fetched looks
// complete and can be submitted entirely blank.
func TestApplicationReadinessBlocksWhenQuestionnaireNeverIngested(
	t *testing.T,
) {
	resumePath := writeReadinessArtifact(t, "resume.txt")

	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			200: {
				ID:                 200,
				JobID:              20,
				Status:             StatusDraft,
				TailoredResumePath: resumePath,
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		&fakeQuestionRepository{},
	).WithEventRepository(&fakeEventRepository{})

	result, err := evaluator.Evaluate(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}

	if result.Ready() {
		t.Fatal(
			"an application whose questionnaire was never ingested must not be ready",
		)
	}

	found := false

	for _, blocker := range result.Blockers {
		if strings.Contains(blocker, "questionnaire has not been ingested") {
			found = true
		}
	}

	if !found {
		t.Fatalf(
			"blockers = %v, want one about missing questionnaire ingestion",
			result.Blockers,
		)
	}
}

// A question marked ANSWERED or APPROVED with an empty answer would be submitted
// blank. Status is not evidence that an answer exists.
func TestApplicationReadinessBlocksBlankAnswers(t *testing.T) {
	testCases := []struct {
		name   string
		status QuestionStatus
	}{
		{name: "answered but blank", status: QuestionAnswered},
		{name: "approved but blank", status: QuestionApproved},
		{name: "whitespace only", status: QuestionAnswered},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resumePath := writeReadinessArtifact(t, "resume.txt")

			answer := ""
			if testCase.name == "whitespace only" {
				answer = "   \n\t "
			}

			applications := &fakeApplicationRepository{
				applications: map[int64]*Application{
					201: {
						ID:                 201,
						JobID:              21,
						Status:             StatusDraft,
						TailoredResumePath: resumePath,
					},
				},
			}

			questions := &fakeQuestionRepository{
				questions: []ApplicationQuestion{
					{
						ID:            9,
						ApplicationID: 201,
						Question:      "What is your notice period?",
						Status:        testCase.status,
						Answer:        answer,
					},
				},
			}

			evaluator := NewApplicationReadinessEvaluator(
				applications,
				questions,
			).WithEventRepository(ingestedQuestionnaireEvents(201))

			result, err := evaluator.Evaluate(context.Background(), 201)
			if err != nil {
				t.Fatal(err)
			}

			if result.Ready() {
				t.Fatalf(
					"a %s question with no answer must block readiness",
					testCase.status,
				)
			}
		})
	}
}

// A recorded path is not evidence the artifact exists.
func TestApplicationReadinessBlocksMissingOrEmptyArtifact(t *testing.T) {
	emptyPath := filepath.Join(t.TempDir(), "empty.txt")

	if err := os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatalf("write empty artifact: %v", err)
	}

	testCases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "file does not exist",
			path: filepath.Join(t.TempDir(), "absent.txt"),
			want: "does not exist",
		},
		{
			name: "file is empty",
			path: emptyPath,
			want: "is empty",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			applications := &fakeApplicationRepository{
				applications: map[int64]*Application{
					202: {
						ID:                 202,
						JobID:              22,
						Status:             StatusDraft,
						TailoredResumePath: testCase.path,
					},
				},
			}

			evaluator := NewApplicationReadinessEvaluator(
				applications,
				&fakeQuestionRepository{},
			).WithEventRepository(ingestedQuestionnaireEvents(202))

			result, err := evaluator.Evaluate(context.Background(), 202)
			if err != nil {
				t.Fatal(err)
			}

			if result.Ready() {
				t.Fatal("a missing or empty artifact must block readiness")
			}

			found := false

			for _, blocker := range result.Blockers {
				if strings.Contains(blocker, testCase.want) {
					found = true
				}
			}

			if !found {
				t.Fatalf(
					"blockers = %v, want one containing %q",
					result.Blockers,
					testCase.want,
				)
			}
		})
	}
}

// LLM answers are not candidate-verified. They are surfaced for review but must
// not block, so the operator decides at the dry-run stage.
func TestApplicationReadinessSurfacesLLMAnswersWithoutBlocking(
	t *testing.T,
) {
	resumePath := writeReadinessArtifact(t, "resume.txt")

	applications := &fakeApplicationRepository{
		applications: map[int64]*Application{
			203: {
				ID:                 203,
				JobID:              23,
				Status:             StatusDraft,
				TailoredResumePath: resumePath,
			},
		},
	}

	questions := &fakeQuestionRepository{
		questions: []ApplicationQuestion{
			{
				ID:            11,
				ApplicationID: 203,
				Question:      "Why this company?",
				Status:        QuestionAnswered,
				Answer:        "Generated rationale",
				AnswerSource:  AnswerSourceLLM,
			},
		},
	}

	evaluator := NewApplicationReadinessEvaluator(
		applications,
		questions,
	).WithEventRepository(ingestedQuestionnaireEvents(203))

	result, err := evaluator.Evaluate(context.Background(), 203)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Ready() {
		t.Fatalf(
			"LLM answers must not block readiness; blockers = %v",
			result.Blockers,
		)
	}

	found := false

	for _, check := range result.Checks {
		if check.Name == "answer_provenance" {
			found = true

			if !strings.Contains(check.Reason, "not candidate-verified") {
				t.Fatalf(
					"provenance reason = %q, want a verification warning",
					check.Reason,
				)
			}
		}
	}

	if !found {
		t.Fatalf(
			"checks = %+v, want an answer_provenance check",
			result.Checks,
		)
	}
}
