CREATE TABLE IF NOT EXISTS candidate_answers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    field_key TEXT NOT NULL UNIQUE,
    question TEXT,
    answer TEXT NOT NULL,
    value_type TEXT NOT NULL DEFAULT 'TEXT',
    verified INTEGER NOT NULL DEFAULT 0,
    notes TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_candidate_answers_field_key
    ON candidate_answers(field_key);

CREATE INDEX IF NOT EXISTS idx_candidate_answers_verified
    ON candidate_answers(verified);
