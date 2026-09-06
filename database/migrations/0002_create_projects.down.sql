-- 0002_create_projects.down.sql
-- Intent: reverse 0002 (test-only / pre-data rollback).
-- Rollback: n/a.
-- Destructive: YES once evidence/audit data references projects.

DROP TABLE IF EXISTS projects;
