package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
)

// ---- users ----

func (d *DB) UserByEmail(ctx context.Context, email string) (auth.User, bool, error) {
	row := d.Pool.QueryRow(ctx, `
		SELECT id, organization_id, role, status, email
		FROM users
		WHERE email = $1
	`, email)
	var u auth.User
	err := row.Scan(&u.ID, &u.OrganizationID, &u.Role, &u.Status, &u.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, false, err
	}
	return u, true, nil
}

// ---- audit log (append-only; no update/delete code path exists) ----

type AuditEntry struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	Action         string
	ResourceType   string
	ResourceID     string
	RequestID      string
	Before         any // marshalled to JSONB; nil → NULL
	After          any
}

func (d *DB) WriteAudit(ctx context.Context, e AuditEntry) error {
	before, err := marshalNullable(e.Before)
	if err != nil {
		return err
	}
	after, err := marshalNullable(e.After)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = d.Pool.Exec(ctx, `
		INSERT INTO audit_log
			(id, actor_id, organization_id, action, resource_type, resource_id, request_id, before_state, after_state)
		VALUES ($1, $2, NULLIF($3::text, '')::uuid, $4, $5, $6, $7, $8::jsonb, $9::jsonb)
	`, id, nullableUUID(e.ActorID), orgIDText(e.OrganizationID), e.Action, e.ResourceType, e.ResourceID, e.RequestID, before, after)
	return err
}

func marshalNullable(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

func nullableUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func orgIDText(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// ---- projects ----

type Project struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	EntityType     string
	TargetLocale   string
	ScopeNote      string
	Status         string
	Version        int
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateProjectInput struct {
	Name         string
	EntityType   string
	TargetLocale string
	ScopeNote    string
}

// CreateProject inserts a project owned by the ACTOR's organization. There is
// deliberately no organization_id parameter: tenancy follows the authenticated
// caller, which makes "trust the request body's organization_id" impossible
// at this layer (execution plan B05 step ③).
func (d *DB) CreateProject(ctx context.Context, actor auth.Actor, in CreateProjectInput) (Project, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Project{}, err
	}
	row := d.Pool.QueryRow(ctx, `
		INSERT INTO projects (id, organization_id, name, entity_type, target_locale, scope_note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`, id, actor.OrganizationID, in.Name, in.EntityType, in.TargetLocale, in.ScopeNote, actor.UserID)

	var p Project
	err = row.Scan(&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	p.ID, p.OrganizationID, p.Name, p.EntityType, p.TargetLocale, p.Status, p.Version, p.CreatedBy = id, actor.OrganizationID, in.Name, in.EntityType, in.TargetLocale, "active", 1, actor.UserID
	return p, nil
}

// ListProjects returns the actor's organization's projects only. The org
// predicate IS the authorization check, not an optimization.
func (d *DB) ListProjects(ctx context.Context, actor auth.Actor) ([]Project, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, organization_id, name, entity_type, target_locale, scope_note, status, version, created_by, created_at, updated_at
		FROM projects
		WHERE organization_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.EntityType, &p.TargetLocale, &p.ScopeNote, &p.Status, &p.Version, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProject is scoped by organization: another tenant's id is indistinguishable
// from a nonexistent id (both ErrNotFound → HTTP 404, no enumeration).
func (d *DB) GetProject(ctx context.Context, actor auth.Actor, projectID uuid.UUID) (Project, error) {
	row := d.Pool.QueryRow(ctx, `
		SELECT id, organization_id, name, entity_type, target_locale, scope_note, status, version, created_by, created_at, updated_at
		FROM projects
		WHERE id = $1 AND organization_id = $2
	`, projectID, actor.OrganizationID)

	var p Project
	err := row.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.EntityType, &p.TargetLocale, &p.ScopeNote, &p.Status, &p.Version, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	return p, nil
}

// UpdateProjectScope is the B05 seam for B06's full PATCH: it bumps version
// optimistically and refuses rows outside the actor's organization.
func (d *DB) UpdateProjectScope(ctx context.Context, actor auth.Actor, projectID uuid.UUID, scopeNote string) (Project, error) {
	row := d.Pool.QueryRow(ctx, `
		UPDATE projects
		SET scope_note = $3, version = version + 1, updated_at = now()
		WHERE id = $1 AND organization_id = $2
		RETURNING id, organization_id, name, entity_type, target_locale, scope_note, status, version, created_by, created_at, updated_at
	`, projectID, actor.OrganizationID, scopeNote)

	var p Project
	err := row.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.EntityType, &p.TargetLocale, &p.ScopeNote, &p.Status, &p.Version, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	return p, nil
}
