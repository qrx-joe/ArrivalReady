-- 0001_create_org_users_audit_log.down.sql
-- Intent: reverse 0001 (test-only / pre-data rollback; see up-file header).
-- Rollback: n/a (this IS the rollback).
-- Destructive: YES once business data exists — never run against a real database
--   without backup + ADR (execution plan §7.3).

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
