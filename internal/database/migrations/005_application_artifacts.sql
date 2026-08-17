ALTER TABLE applications
    ADD COLUMN tailored_resume_path TEXT;

ALTER TABLE applications
    ADD COLUMN cover_letter_path TEXT;

ALTER TABLE applications
    ADD COLUMN referral_message_path TEXT;

ALTER TABLE applications
    ADD COLUMN application_answers_path TEXT;

ALTER TABLE applications
    ADD COLUMN updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE applications
SET status = 'DRAFT'
WHERE status = 'PENDING';
