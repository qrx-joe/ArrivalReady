package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// User is the tenant-scoped identity row. Organization and role come from OUR
// database, not from token claims (see package doc).
type User struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Role           string
	Status         string
	Email          string
}

// UserLookup is the persistence dependency of the middleware.
type UserLookup interface {
	UserByEmail(ctx context.Context, email string) (User, bool, error)
}

// Actor is the authenticated principal attached to every request that passes
// the middleware; repositories MUST scope queries by OrganizationID.
type Actor struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           string
	Email          string
}

type actorContextKey struct{}

// ActorFrom returns the authenticated actor. ok=false means the handler was
// reached without passing the auth middleware — a wiring bug, never allowed.
func ActorFrom(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}

// Middleware authenticates the bearer token and attaches the Actor. Identity
// decisions return 401 (never 403): an unrecognized or disabled identity must
// be indistinguishable from "no token" to avoid account enumeration.
func Middleware(authn Authenticator, users UserLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				unauthorized(w, "missing bearer token")
				return
			}
			claims, err := authn.Verify(r.Context(), raw)
			if err != nil {
				unauthorized(w, "invalid token")
				return
			}
			user, found, err := users.UserByEmail(r.Context(), claims.Email)
			if err != nil {
				writeProblem(w, http.StatusInternalServerError, "lookup failed")
				return
			}
			// Token proves the email, but only an active user row proves
			// membership: unknown or disabled identities cannot act at all.
			if !found || user.Status != "active" {
				unauthorized(w, "identity not recognized")
				return
			}

			actor := Actor{
				UserID:         user.ID,
				OrganizationID: user.OrganizationID,
				Role:           user.Role,
				Email:          user.Email,
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorContextKey{}, actor)))
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

func unauthorized(w http.ResponseWriter, detail string) {
	writeProblem(w, http.StatusUnauthorized, detail)
}

func writeProblem(w http.ResponseWriter, code int, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  http.StatusText(code),
		"status": code,
		"detail": detail,
	})
}

// ---- Local test identity (B05 step ⑤) ----

// TestIdentityConfig captures the two switches that gate the local test
// identity. The gate IS the security boundary: the identity must be
// impossible to reach in production even by misconfiguration accident.
type TestIdentityConfig struct {
	Env        string // must be exactly "dev"
	EnableFlag string // must be exactly "1" (opt-in, never default-on)
	Secret     string // HS256 secret, >= 32 bytes
}

// NewTestAuthenticator returns an HS256-backed Authenticator for local tests.
// It REFUSES to construct unless every gate condition holds; a refusal is a
// startup error, not a warning (execution plan B05: 真实数据路径禁止绕过).
func NewTestAuthenticator(cfg TestIdentityConfig) (Authenticator, error) {
	if cfg.Env != "dev" || cfg.EnableFlag != "1" {
		return nil, errors.New(
			"test identity refused: requires ENV=dev AND ARRIVAL_ENABLE_TEST_IDENTITY=1 " +
				"(real-data paths must never accept local identities)")
	}
	if len(cfg.Secret) < 32 {
		return nil, errors.New("test identity refused: ARRIVAL_TEST_IDENTITY_SECRET must be >= 32 bytes")
	}
	return &testAuthenticator{secret: []byte(cfg.Secret)}, nil
}

type testAuthenticator struct {
	secret []byte
}

// Sign mints a local test token. It exists ONLY on the test authenticator:
// real OIDC identities come from the external IdP, never from our code.
func (t *testAuthenticator) Sign(subject, email string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":   subject,
		"email": email,
		"iss":   "arrivalready-test",
		"aud":   "arrivalready-dev",
		"exp":   time.Now().Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *testAuthenticator) Verify(_ context.Context, rawToken string) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(rawToken, jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing algorithm %q, want HS256", token.Method.Alg())
		}
		return t.secret, nil
	},
		jwt.WithIssuer("arrivalready-test"),
		jwt.WithAudience("arrivalready-dev"),
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("token rejected: %w", err)
	}
	return claimsFromToken(parsed)
}
