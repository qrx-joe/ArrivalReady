-- 0004_create_idempotency_keys.down.sql
-- Intent: reverse 0004（test-only rollback；幂等历史丢失可接受时才允许）。
-- Rollback: n/a.
-- Destructive: no

DROP TABLE IF EXISTS idempotency_keys;
