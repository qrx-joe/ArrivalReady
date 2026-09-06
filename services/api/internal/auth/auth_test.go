package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ---- test doubles ----

type fakeUsers struct {
	user  User
	found bool
	err   error
}

func (f fakeUsers) UserByEmail(_ context.Context, _ string) (User, bool, error) {
	return f.user, f.found, f.err
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

// jwksServer serves exactly one RSA key under kid "test-key".
func jwksServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := &key.PublicKey
		_ = json.NewEncoder(w).Encode(jwksDocument{Keys: []JWK{{
			Kty: "RSA", Kid: "test-key", Alg: "RS256", Use: "sig",
			N: base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	return httptest.NewServer(mux)
}

func signRS256(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "test-key"
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func baseClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"sub":   "idp-user-1",
		"email": "alice@org-a.test",
		"iss":   "https://idp.test",
		"aud":   "arrivalready",
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	}
}

func newOIDC(t *testing.T, jwksURL string) *OIDCAuthenticator {
	t.Helper()
	return NewOIDCAuthenticator("https://idp.test", "arrivalready", jwksURL+"/jwks", nil)
}

// ---- OIDC verification ----

func TestVerifyAcceptsValidToken(t *testing.T) {
	key := mustRSAKey(t)
	srv := jwksServer(t, key)
	defer srv.Close()

	claims, err := newOIDC(t, srv.URL).Verify(context.Background(), signRS256(t, key, baseClaims()))
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if claims.Email != "alice@org-a.test" || claims.Subject != "idp-user-1" {
		t.Fatalf("wrong claims: %+v", claims)
	}
}

func TestVerifyRejections(t *testing.T) {
	key := mustRSAKey(t)
	srv := jwksServer(t, key)
	defer srv.Close()
	authn := newOIDC(t, srv.URL)

	cases := []struct {
		name  string
		token func() string
	}{
		{"expired", func() string {
			c := baseClaims()
			c["exp"] = time.Now().Add(-time.Minute).Unix()
			return signRS256(t, key, c)
		}},
		{"wrong audience", func() string {
			c := baseClaims()
			c["aud"] = "someone-else"
			return signRS256(t, key, c)
		}},
		{"wrong issuer", func() string {
			c := baseClaims()
			c["iss"] = "https://evil.test"
			return signRS256(t, key, c)
		}},
		{"forged signature", func() string {
			other := mustRSAKey(t)
			return signRS256(t, other, baseClaims())
		}},
		{"alg confusion (HS256)", func() string {
			tok := jwt.NewWithClaims(jwt.SigningMethodHS256, baseClaims())
			tok.Header["kid"] = "test-key"
			s, _ := tok.SignedString([]byte("attacker-controlled-secret-not-a-rsa-key"))
			return s
		}},
		{"missing email claim", func() string {
			c := baseClaims()
			delete(c, "email")
			return signRS256(t, key, c)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := authn.Verify(context.Background(), tc.token()); err == nil {
				t.Fatalf("%s: expected rejection, got valid claims", tc.name)
			}
		})
	}
}

// ---- middleware ----

func handlerCapturingActor(t *testing.T, captured *bool) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ActorFrom(r.Context()); !ok {
			t.Error("handler reached without actor in context")
		}
		*captured = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestMiddlewareChain(t *testing.T) {
	key := mustRSAKey(t)
	srv := jwksServer(t, key)
	defer srv.Close()
	authn := newOIDC(t, srv.URL)

	orgA := uuid.Must(uuid.NewV7())
	users := fakeUsers{user: User{
		ID: uuid.Must(uuid.NewV7()), OrganizationID: orgA,
		Role: "MEMBER", Status: "active", Email: "alice@org-a.test",
	}, found: true}
	mw := Middleware(authn, users)

	do := func(token string) *httptest.ResponseRecorder {
		var reached bool
		req := httptest.NewRequest(http.MethodGet, "/projects", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mw(handlerCapturingActor(t, &reached)).ServeHTTP(rec, req)
		if reached && rec.Code != http.StatusOK {
			t.Fatalf("handler reached but status %d", rec.Code)
		}
		return rec
	}

	if rec := do(""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", rec.Code)
	}
	if rec := do(signRS256(t, key, baseClaims())); rec.Code != http.StatusOK {
		t.Fatalf("valid token: got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	disabled := fakeUsers{user: User{Status: "disabled"}, found: true}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("Authorization", "Bearer "+signRS256(t, key, baseClaims()))
	Middleware(authn, disabled)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("disabled user reached handler")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("disabled user: got %d, want 401", rec.Code)
	}

	unknown := fakeUsers{found: false}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("Authorization", "Bearer "+signRS256(t, key, baseClaims()))
	Middleware(authn, unknown)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("unknown user reached handler")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user: got %d, want 401", rec.Code)
	}
}

// ---- local test identity gate ----

func TestTestIdentityGate(t *testing.T) {
	if _, err := NewTestAuthenticator(TestIdentityConfig{Env: "production", EnableFlag: "1", Secret: strings.Repeat("s", 32)}); err == nil {
		t.Fatal("production must refuse the local test identity")
	}
	if _, err := NewTestAuthenticator(TestIdentityConfig{Env: "dev", EnableFlag: "", Secret: strings.Repeat("s", 32)}); err == nil {
		t.Fatal("unset flag must refuse the local test identity")
	}
	if _, err := NewTestAuthenticator(TestIdentityConfig{Env: "dev", EnableFlag: "1", Secret: "short"}); err == nil {
		t.Fatal("short secret must refuse the local test identity")
	}

	authn, err := NewTestAuthenticator(TestIdentityConfig{Env: "dev", EnableFlag: "1", Secret: strings.Repeat("s", 32)})
	if err != nil {
		t.Fatalf("dev opt-in should construct: %v", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "local-tester", "email": "tester@local.test",
		"iss": "arrivalready-test", "aud": "arrivalready-dev",
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	signed, _ := tok.SignedString([]byte(strings.Repeat("s", 32)))
	if _, err := authn.Verify(context.Background(), signed); err != nil {
		t.Fatalf("local test token rejected: %v", err)
	}
}

func TestParseMalformedToken(t *testing.T) {
	authn := NewOIDCAuthenticator("https://idp.test", "arrivalready", "http://127.0.0.1:1/jwks", nil)
	if _, err := authn.Verify(context.Background(), "not.a.token"); err == nil {
		t.Fatal("malformed token must be rejected")
	}
}
