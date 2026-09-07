-- 0005_create_audit_jobs.down.sql
-- Intent: reverse 0005（test-only / pre-data rollback）。
-- Rollback: n/a.
-- Destructive: YES once review/scoring data exists。

DROP TABLE IF EXISTS finding_evidence;
DROP TABLE IF EXISTS findings;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS audit_runs;
