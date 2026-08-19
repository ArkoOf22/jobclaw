CREATE UNIQUE INDEX IF NOT EXISTS
idx_application_questions_application_question
ON application_questions(application_id, question);
