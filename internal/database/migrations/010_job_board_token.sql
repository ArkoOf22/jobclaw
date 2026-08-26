-- Persist the ATS board token a job was discovered from.
--
-- The token is authoritative discovery-time knowledge: the Greenhouse client is
-- configured with it and uses it to fetch the job. It was then discarded, so
-- later stages had to re-derive it by parsing the job URL. That only works for
-- greenhouse.io-hosted boards. Employers commonly front their board on their own
-- domain, for example https://stripe.com/jobs/search?gh_jid=6042172, where the
-- token is absent from the URL entirely, which made fetching the application
-- form impossible without an out-of-band guess.
--
-- Nullable because jobs discovered before this migration have no recorded token,
-- and sources other than Greenhouse do not have one.

ALTER TABLE jobs
    ADD COLUMN board_token TEXT;
