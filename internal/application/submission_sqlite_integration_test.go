package application

import (
	"context"
	"errors"
	"testing"

	"jobclaw/internal/database"
	"jobclaw/internal/job"
)

type sqliteIntegrationSubmitter struct {
	err         error
	submissions int
}

func (s *sqliteIntegrationSubmitter) Submit(
	ctx context.Context,
	app Application,
	j job.Job,
) error {
	s.submissions++
	return s.err
}

func setupSubmissionIntegrationDB(t *testing.T) *database.DB {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := db.MigrateEmbedded(); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return db
}

func seedSubmissionFixture(
	t *testing.T,
	db *database.DB,
) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO job_sources (
			name
		)
		VALUES (?)
	`, "integration")
	if err != nil {
		t.Fatalf("insert job source: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO companies (
			name,
			normalized_name
		)
		VALUES (?, ?)
	`, "Integration Company", "integration company")
	if err != nil {
		t.Fatalf("insert company: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO jobs (
			source_id,
			external_id,
			company,
			title,
			description,
			location,
			url,
			status,
			company_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		1,
		"integration-job-1",
		"Integration Company",
		"Backend Engineer",
		"Backend engineering role for integration testing",
		"Bangalore, India",
		"https://integration.example/jobs/1",
		job.StatusApproved,
		1,
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO applications (
			job_id,
			status,
			tailored_resume_path
		)
		VALUES (?, ?, ?)
	`,
		1,
		StatusReadyToApply,
		"resume.txt",
	)
	if err != nil {
		t.Fatalf("insert application: %v", err)
	}
}

func TestSQLiteApplicationSubmissionCommitsAtomically(
	t *testing.T,
) {
	db := setupSubmissionIntegrationDB(t)
	seedSubmissionFixture(t, db)

	applicationRepo := NewSQLiteRepository(db)
	jobRepo := job.NewSQLiteRepository(db)
	eventRepo := NewSQLiteEventRepository(db)

	submitter := &sqliteIntegrationSubmitter{}

	service := NewApplicationSubmissionService(
		applicationRepo,
		jobRepo,
		eventRepo,
		submitter,
	)

	service.SetTransactionFactory(
		NewSQLiteSubmissionTransactionFactory(db),
	)

	if err := service.Submit(context.Background(), 1); err != nil {
		t.Fatalf("submit application: %v", err)
	}

	if submitter.submissions != 1 {
		t.Fatalf(
			"submissions = %d, want 1",
			submitter.submissions,
		)
	}

	app, err := applicationRepo.GetByID(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf("load application: %v", err)
	}

	if app.Status != StatusApplied {
		t.Fatalf(
			"application status = %q, want %q",
			app.Status,
			StatusApplied,
		)
	}

	j, err := jobRepo.GetByID(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}

	if j.Status != job.StatusApplied {
		t.Fatalf(
			"job status = %q, want %q",
			j.Status,
			job.StatusApplied,
		)
	}

	var eventCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM application_events
		WHERE application_id = ?
		  AND event_type = ?
	`,
		1,
		EventApplicationSubmitted,
	).Scan(&eventCount)

	if err != nil {
		t.Fatalf("count submission events: %v", err)
	}

	if eventCount != 1 {
		t.Fatalf(
			"submission events = %d, want 1",
			eventCount,
		)
	}
}

func TestSQLiteApplicationSubmissionRollsBackAtomically(
	t *testing.T,
) {
	db := setupSubmissionIntegrationDB(t)
	seedSubmissionFixture(t, db)

	applicationRepo := NewSQLiteRepository(db)
	jobRepo := job.NewSQLiteRepository(db)
	eventRepo := NewSQLiteEventRepository(db)

	submitter := &sqliteIntegrationSubmitter{}

	service := NewApplicationSubmissionService(
		applicationRepo,
		jobRepo,
		eventRepo,
		submitter,
	)

	service.SetTransactionFactory(
		&failingSQLiteSubmissionTransactionFactory{
			db:  db,
			err: errors.New("forced job update failure"),
		},
	)

	err := service.Submit(context.Background(), 1)
	if err == nil {
		t.Fatal("submit application succeeded, want error")
	}

	if submitter.submissions != 1 {
		t.Fatalf(
			"submissions = %d, want 1",
			submitter.submissions,
		)
	}

	app, err := applicationRepo.GetByID(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf("load application: %v", err)
	}

	if app.Status != StatusSubmissionInProgress {
		t.Fatalf(
			"application status = %q, want %q after local submission failure",
			app.Status,
			StatusSubmissionInProgress,
		)
	}

	j, err := jobRepo.GetByID(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}

	if j.Status != job.StatusApproved {
		t.Fatalf(
			"job status = %q, want %q after rollback",
			j.Status,
			job.StatusApproved,
		)
	}

	var eventCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM application_events
		WHERE application_id = ?
		  AND event_type = ?
	`,
		1,
		EventApplicationSubmitted,
	).Scan(&eventCount)

	if err != nil {
		t.Fatalf("count submission events: %v", err)
	}

	if eventCount != 0 {
		t.Fatalf(
			"submission events = %d, want 0 after local submission failure",
			eventCount,
		)
	}
}

type failingSQLiteSubmissionTransactionFactory struct {
	db  *database.DB
	err error
}

func (f *failingSQLiteSubmissionTransactionFactory) BeginSubmissionTransaction(
	ctx context.Context,
) (SubmissionTransaction, error) {
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &failingSQLiteSubmissionTransaction{
		sqliteSubmissionTransaction: &sqliteSubmissionTransaction{
			tx: tx,
		},
		err: f.err,
	}, nil
}

type failingSQLiteSubmissionTransaction struct {
	*sqliteSubmissionTransaction
	err error
}

func (t *failingSQLiteSubmissionTransaction) UpdateJobStatus(
	ctx context.Context,
	id int64,
	status job.Status,
) error {
	return t.err
}
