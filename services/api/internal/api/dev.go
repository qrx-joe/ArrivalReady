// Dev-only identity minting for the local web workflow (execution plan B09).
//
// Security gate: routes in this file are registered ONLY when the test
// identity authenticator is active (ENV=dev AND ARRIVAL_ENABLE_TEST_IDENTITY=1,
// enforced in main). With any other configuration this handler does not exist
// at all — there is no runtime check to bypass. Tokens it mints are the same
// HS256 test tokens auth.NewTestAuthenticator verifies.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/google/uuid"

	"github.com/qrx-joe/ArrivalReady/services/api/internal/auth"
	"github.com/qrx-joe/ArrivalReady/services/api/internal/store"
)

// DevTokenSigner mints test-identity tokens; implemented by
// auth.TestAuthenticator (Sign) alongside its Verify.
type DevTokenSigner interface {
	Sign(subject, email string, ttl time.Duration) (string, error)
}

type DevIdentity struct {
	DB     *store.DB
	Signer DevTokenSigner
	Email  string // default dev identity
}

func (d *DevIdentity) RegisterRoutes(r chi.Router) {
	r.HandleFunc("POST /internal/dev-token", d.devToken)
}

// devToken ensures the dev organization + user exist, then mints a token for
// them. Idempotent: repeated logins reuse the same rows.
func (d *DevIdentity) devToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil || in.Email == "" {
		in.Email = d.Email
	}

	ctx := r.Context()
	actor, err := d.ensureDevUser(ctx, in.Email)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "dev identity bootstrap failed")
		return
	}
	token, err := d.Signer.Sign("local-tester:"+in.Email, in.Email, 12*time.Hour)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "token minting failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":           token,
		"email":           actor.Email,
		"organization_id": actor.OrganizationID,
		"user_id":         actor.UserID,
	})
}

func (d *DevIdentity) ensureDevUser(ctx context.Context, email string) (auth.Actor, error) {
	if u, found, err := d.DB.UserByEmail(ctx, email); err == nil && found {
		return auth.Actor{UserID: u.ID, OrganizationID: u.OrganizationID, Role: u.Role, Email: u.Email}, nil
	}

	// organizations.name has no unique index; look before insert so repeated
	// dev logons reuse one tenant instead of spawning duplicates.
	var orgID uuid.UUID
	err := d.DB.Pool.QueryRow(ctx,
		`SELECT id FROM organizations WHERE name = $1`, "Dev Organization").Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		orgID, err = uuid.NewV7()
		if err != nil {
			return auth.Actor{}, err
		}
		if _, ierr := d.DB.Pool.Exec(ctx,
			`INSERT INTO organizations (id, name) VALUES ($1, $2)`, orgID, "Dev Organization"); ierr != nil {
			return auth.Actor{}, ierr
		}
	} else if err != nil {
		return auth.Actor{}, err
	}
	userID, err := uuid.NewV7()
	if err != nil {
		return auth.Actor{}, err
	}
	_, err = d.DB.Pool.Exec(ctx, `
		INSERT INTO users (id, organization_id, role, email) VALUES ($1,$2,'OWNER',$3)
	`, userID, orgID, email)
	if err != nil {
		return auth.Actor{}, err
	}
	return auth.Actor{UserID: userID, OrganizationID: orgID, Role: "OWNER", Email: email}, nil
}
