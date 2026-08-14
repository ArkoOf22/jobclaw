ALTER TABLE job_scores
    ADD COLUMN role_score REAL;

ALTER TABLE job_scores
    ADD COLUMN domain_score REAL;

ALTER TABLE job_scores
    ADD COLUMN company_score REAL;
