CREATE TABLE IF NOT EXISTS application_questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    application_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    field_key TEXT,
    answer TEXT,
    answer_source TEXT,
    status TEXT NOT NULL DEFAULT 'NEEDS_REVIEW',
    metadata TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (application_id)
        REFERENCES applications(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_application_questions_application_id
    ON application_questions(application_id);

CREATE INDEX IF NOT EXISTS idx_application_questions_status
    ON application_questions(status);
