-- 0006_review_score_task.down.sql
-- Intent: reverse 0006（test-only / pre-data rollback）。
-- Destructive: YES once review/scoring/task data exists。

DROP TABLE IF EXISTS reviews;
ALTER TABLE audit_runs DROP COLUMN IF EXISTS scoring;
DROP TABLE IF EXISTS fix_task_events;
DROP TABLE IF EXISTS fix_tasks;
ALTER TABLE findings DROP COLUMN IF EXISTS effective_status;
ALTER TABLE findings DROP COLUMN IF EXISTS na_reason;
