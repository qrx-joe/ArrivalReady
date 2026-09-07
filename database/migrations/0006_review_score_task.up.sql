-- 0006_review_score_task.up.sql
-- Intent: 人审记录（append-only）、finding 生效结论列、报告冻结列、整改任务
--   与状态迁移事件（B10/B11，ADR-0002 + 状态迁移表）。
-- Rollback: DROP 逆序（safe before 演示数据；之后 destructive → backup + ADR）。
-- Destructive: no

ALTER TABLE findings ADD COLUMN effective_status text
    CHECK (effective_status IN ('PASS','WARN','FAIL','UNKNOWN','NA'));
ALTER TABLE findings ADD COLUMN na_reason text;
UPDATE findings SET effective_status = assessment_status WHERE review_status = 'UNREVIEWED';

-- 报告冻结列（B10 finalize 原子写入；完成后只读）
ALTER TABLE audit_runs ADD COLUMN scoring jsonb;

-- 人审记录（append-only，R-05：edit 保留原始候选，历史不覆盖）
CREATE TABLE reviews (
    id          uuid PRIMARY KEY,
    finding_id  uuid        NOT NULL REFERENCES findings (id),
    reviewer_id uuid        NOT NULL REFERENCES users (id),
    decision    text        NOT NULL CHECK (decision IN ('confirm','reject','edit','na')),
    note        text,
    na_reason   text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- B11: 整改任务主表（活动状态投影，不覆盖历史判断）+ 每次迁移的事件行
CREATE TABLE fix_tasks (
    id              uuid PRIMARY KEY,
    finding_id      uuid        NOT NULL UNIQUE REFERENCES findings (id),
    organization_id uuid        NOT NULL REFERENCES organizations (id),
    workflow_status text        NOT NULL DEFAULT 'OPEN'
        CHECK (workflow_status IN ('OPEN','ACKNOWLEDGED','FIXING','READY_FOR_RETEST','RESOLVED','ACCEPTED_RISK','REOPENED')),
    owner_id        uuid        REFERENCES users (id),
    reason          text,
    version         integer     NOT NULL DEFAULT 1,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE fix_task_events (
    id          uuid PRIMARY KEY,
    task_id     uuid        NOT NULL REFERENCES fix_tasks (id),
    actor_id    uuid        REFERENCES users (id),
    from_status text,
    to_status   text        NOT NULL,
    reason      text,
    request_id  text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
