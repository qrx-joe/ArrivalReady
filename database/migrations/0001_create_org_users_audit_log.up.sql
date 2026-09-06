-- 0001_create_org_users_audit_log.up.sql
-- Intent: baseline tenancy tables (organization, user) and append-only audit log
--   per TECH_SPEC §5.1/§5.2/§32 and execution-plan B05.
-- Rollback: DROP TABLE audit_log, users, organizations (safe only before any
--   business tables reference users/organizations; after B06 this is
--   destructive → backup + ADR required).
-- Destructive: no

CREATE TABLE organizations (
    id         uuid PRIMARY KEY,
    name       text        NOT NULL,
    plan       text        NOT NULL DEFAULT 'trial'
        CHECK (plan IN ('trial', 'standard')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id              uuid PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organizations (id),
    role            text        NOT NULL
        CHECK (role IN ('OWNER', 'ADMIN', 'REVIEWER', 'MEMBER', 'VIEWER')),
    email           text        NOT NULL,
    status          text        NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- For the MVP the verified email from the identity token is the identity
-- handle that maps JWT claims to a user row; it must be unique per tenant.
CREATE UNIQUE INDEX users_email_uniq_idx ON users ((email));
CREATE INDEX users_organization_idx ON users (organization_id);

-- Append-only audit trail. No UPDATE/DELETE path exists in application code;
-- if retention ever requires pruning that is a separate, authorized process
-- (execution plan §7.3), never an inline write.
CREATE TABLE audit_log (
    id              uuid        PRIMARY KEY,
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    actor_id        uuid        REFERENCES users (id),
    organization_id uuid,
    action          text        NOT NULL,
    resource_type   text        NOT NULL,
    resource_id     text        NOT NULL,
    request_id      text,
    before_state    jsonb,
    after_state     jsonb
);

CREATE INDEX audit_log_org_time_idx ON audit_log (organization_id, occurred_at DESC);
CREATE INDEX audit_log_resource_idx ON audit_log (resource_type, resource_id);
