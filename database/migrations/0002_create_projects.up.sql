-- 0002_create_projects.up.sql
-- Intent: 验收项目表（PRD P0-1 / US-01）。每个项目只属于一个 organization；
--   所有读写必须带 organization_id 过滤（BOLA 防御，OWASP API1；TECH_SPEC §12
--   禁止仅凭客户端提供的项目 id 放行）。
-- Rollback: DROP TABLE projects (safe before evidence/audit FKs land in B06+;
--   afterwards destructive → backup + ADR).
-- Destructive: no

CREATE TABLE projects (
    id              uuid PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organizations (id),
    name            text        NOT NULL,
    entity_type     text        NOT NULL
        CHECK (entity_type IN ('restaurant', 'retail', 'museum', 'venue', 'other')),
    target_locale   text        NOT NULL,
    scope_note      text,
    status          text        NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    version         integer     NOT NULL DEFAULT 1,
    created_by      uuid        NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- The workhorse isolation predicate: listing and lookup always combine
-- organization_id with the actor's org, so this index serves authorization.
CREATE INDEX projects_org_idx ON projects (organization_id, status);
