CREATE UNIQUE INDEX IF NOT EXISTS idx_job_scores_job_id_unique
    ON job_scores(job_id);
