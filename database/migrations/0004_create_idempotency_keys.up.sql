-- 0004_create_idempotency_keys.up.sql
-- Intent: 幂等键存储（TECH_SPEC §11）。按组织 + 键唯一；fingerprint 是请求体
--   规范化 hash——同 key 同 fingerprint 重放保存的响应，不同 fingerprint 视为
--   冲突（409），绝不静默重跑。
-- Rollback: DROP TABLE（独立表，无业务外键，安全）。
-- Destructive: no

CREATE TABLE idempotency_keys (
    id              uuid PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organizations (id),
    key             text        NOT NULL,
    endpoint        text        NOT NULL,
    fingerprint     text        NOT NULL,
    response_status integer     NOT NULL,
    response_body   jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, key, endpoint)
);
