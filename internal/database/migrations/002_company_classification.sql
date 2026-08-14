CREATE TABLE IF NOT EXISTS companies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL UNIQUE,
    classification TEXT NOT NULL DEFAULT 'UNKNOWN',
    classification_reason TEXT,
    classified_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_companies_classification
    ON companies(classification);

ALTER TABLE jobs
    ADD COLUMN company_id INTEGER
    REFERENCES companies(id);

CREATE INDEX IF NOT EXISTS idx_jobs_company_id
    ON jobs(company_id);
