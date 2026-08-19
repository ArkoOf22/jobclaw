ALTER TABLE applications
    ADD COLUMN submission_attempted_at TEXT;

ALTER TABLE applications
    ADD COLUMN submission_attempt_id TEXT;

CREATE INDEX IF NOT EXISTS idx_applications_submission_attempt_id
    ON applications(submission_attempt_id);
