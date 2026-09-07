-- 0005_create_audit_jobs.up.sql
-- Intent: AuditRun 与可恢复 job（TECH_SPEC §5.7/§10、R-03/R-07/R-11、B07）。
--   run 冻结标准版本与证据输入清单；job 用租约 + attempt token 独占执行权，
--   过期租约的旧 attempt 提交一律丢弃；Findings 与 run 状态在事务内原子落库。
-- Rollback: DROP TABLE finding_evidence, findings, jobs, audit_runs（safe
--   before 评分/任务表 B10/B11；之后 destructive → backup + ADR）。
-- Destructive: no

CREATE TABLE audit_runs (
    id                    uuid PRIMARY KEY,
    project_id            uuid        NOT NULL REFERENCES projects (id),
    organization_id       uuid        NOT NULL REFERENCES organizations (id),
    parent_run_id         uuid        REFERENCES audit_runs (id),
    status                text        NOT NULL DEFAULT 'QUEUED'
        CHECK (status IN ('DRAFT','QUEUED','INGESTING','ANALYZING','REVIEW_REQUIRED','COMPLETED','FAILED','CANCELLED')),
    status_reason         text,
    standard_code         text        NOT NULL DEFAULT 'IRRS',
    standard_version      text        NOT NULL,
    rules_sha256          text        NOT NULL,
    evidence_manifest     jsonb       NOT NULL,
    model_bundle_version  text,
    prompt_bundle_version text,
    created_by            uuid        NOT NULL REFERENCES users (id),
    created_at            timestamptz NOT NULL DEFAULT now(),
    completed_at          timestamptz
);

CREATE INDEX audit_runs_org_idx ON audit_runs (organization_id, created_at DESC);

CREATE TABLE jobs (
    id             uuid PRIMARY KEY,
    run_id         uuid        NOT NULL UNIQUE REFERENCES audit_runs (id),
    organization_id uuid       NOT NULL REFERENCES organizations (id),
    status         text        NOT NULL DEFAULT 'QUEUED'
        CHECK (status IN ('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
    attempts       integer     NOT NULL DEFAULT 0,
    max_attempts   integer     NOT NULL DEFAULT 3,
    attempt_token  uuid,
    lease_until    timestamptz,
    last_error     text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX jobs_queued_idx ON jobs (status, created_at) WHERE status = 'QUEUED';

CREATE TABLE findings (
    id                 uuid PRIMARY KEY,
    audit_run_id       uuid        NOT NULL REFERENCES audit_runs (id),
    organization_id    uuid        NOT NULL REFERENCES organizations (id),
    rule_id            text        NOT NULL,
    assessment_status  text        NOT NULL
        CHECK (assessment_status IN ('PASS','WARN','FAIL','UNKNOWN')),
    severity           text        NOT NULL CHECK (severity IN ('S0','S1','S2','S3','Info')),
    title              text,
    observation        text,
    reason             text,
    recommended_fix    text,
    confidence         numeric(4,3) NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    ai_generated       boolean     NOT NULL DEFAULT true,
    review_status      text        NOT NULL DEFAULT 'UNREVIEWED'
        CHECK (review_status IN ('UNREVIEWED','CONFIRMED','EDITED','REJECTED','NA')),
    original_candidate jsonb,
    created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX findings_run_idx ON findings (audit_run_id);
CREATE UNIQUE INDEX findings_run_rule_uniq ON findings (audit_run_id, rule_id);

CREATE TABLE finding_evidence (
    finding_id   uuid  NOT NULL REFERENCES findings (id),
    evidence_id  uuid  NOT NULL REFERENCES evidence (id),
    locator      jsonb NOT NULL,
    PRIMARY KEY (finding_id, evidence_id)
);
