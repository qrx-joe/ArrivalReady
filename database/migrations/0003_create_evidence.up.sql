-- 0003_create_evidence.up.sql
-- Intent: 证据表（PRD P0-2 / US-02）。每条证据冗余 organization_id 以便所有
--   查询带租户过滤（与 projects 同一 BOLA 防御）；processing_status 只有 READY
--   可进入 AI 输入清单（B06 verify：未完成/隔离对象不能送 AI）。
-- Rollback: DROP TABLE evidence（safe before audit_runs 引用落地 B07；之后
--   destructive → backup + ADR）。
-- Destructive: no

CREATE TABLE evidence (
    id                 uuid PRIMARY KEY,
    project_id         uuid        NOT NULL REFERENCES projects (id),
    organization_id    uuid        NOT NULL REFERENCES organizations (id),
    type               text        NOT NULL
        CHECK (type IN ('image', 'pdf', 'url', 'text')),
    source_uri_note    text,
    object_key         text,
    sha256             text,
    mime_type          text,
    size_bytes         bigint,
    journey_stage      text        NOT NULL DEFAULT 'unspecified'
        CHECK (journey_stage IN
            ('discover', 'understand', 'decide', 'act', 'pay', 'recover', 'remember', 'unspecified')),
    processing_status  text        NOT NULL DEFAULT 'PENDING_UPLOAD'
        CHECK (processing_status IN
            ('PENDING_UPLOAD', 'VALIDATING', 'READY', 'QUARANTINED', 'DELETED')),
    quarantine_reason  text,
    -- fingerprint of the LAST complete-upload request; enables exact-replay
    -- idempotency vs conflicting re-completion (TECH_SPEC §11).
    fingerprint        text,
    captured_at        timestamptz,
    created_by         uuid        NOT NULL REFERENCES users (id),
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX evidence_org_status_idx ON evidence (organization_id, processing_status);
CREATE INDEX evidence_project_idx ON evidence (project_id);

-- 0004_create_idempotency_keys.up.sql 会复用 evidence 的唯一归属模型；
-- upload complete 的幂等记录独立建表，便于按组织设置过期策略。
CREATE UNIQUE INDEX evidence_object_key_uniq ON evidence (object_key) WHERE object_key IS NOT NULL;
