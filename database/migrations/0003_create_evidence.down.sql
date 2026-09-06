-- 0003_create_evidence.down.sql
-- Intent: reverse 0003（test-only / pre-data rollback）。
-- Rollback: n/a.
-- Destructive: YES once audit runs reference evidence。

DROP TABLE IF EXISTS evidence;
