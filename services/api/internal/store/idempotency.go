package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Idempotency store (TECH_SPEC §11). The unique index (organization_id, key,
// endpoint) makes concurrent duplicate submissions race-safe: only the first
// response is stored; exact replays return it, divergent payloads conflict.

type StoredResponse struct {
	Status      int
	Body        []byte
	Fingerprint string
}

func Fingerprint(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (d *DB) LookupIdempotent(ctx context.Context, orgID uuid.UUID, key, endpoint string) (StoredResponse, bool, error) {
	row := d.Pool.QueryRow(ctx, `
		SELECT response_status, response_body, fingerprint
		FROM idempotency_keys
		WHERE organization_id = $1 AND key = $2 AND endpoint = $3
	`, orgID, key, endpoint)
	var resp StoredResponse
	var body []byte
	err := row.Scan(&resp.Status, &body, &resp.Fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredResponse{}, false, nil
	}
	if err != nil {
		return StoredResponse{}, false, err
	}
	resp.Body = body
	return resp, true, nil
}

// SaveIdempotent returns false when the (org, key, endpoint) slot is already
// taken; callers then re-lookup and compare fingerprints.
func (d *DB) SaveIdempotent(ctx context.Context, orgID uuid.UUID, key, endpoint, fingerprint string, status int, body []byte) (bool, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return false, err
	}
	tag, err := d.Pool.Exec(ctx, `
		INSERT INTO idempotency_keys
			(id, organization_id, key, endpoint, fingerprint, response_status, response_body)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (organization_id, key, endpoint) DO NOTHING
	`, id, orgID, key, endpoint, fingerprint, status, body)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
