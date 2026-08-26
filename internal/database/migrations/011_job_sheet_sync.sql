-- Track when a job was written to the review sheet.
--
-- The sheet is append-only and is worked through by hand, so a job must appear
-- exactly once. Recording the sync locally is cheaper and more reliable than
-- reading the sheet back to diff it, and it keeps the sheet a pure output rather
-- than a source of truth.
--
-- Nullable: NULL means not yet written.

ALTER TABLE jobs
    ADD COLUMN sheet_synced_at TEXT;
