// Package auth implements OIDC-backed authentication and the actor context
// used for every authorization decision (execution plan B05).
//
// Invariants:
//   - The token only ever proves IDENTITY (subject/email). Organization and
//     role are loaded from OUR database — never from token claims — because
//     tenancy changes must take effect without waiting for token refresh.
//   - Every protected handler receives an Actor or the request is rejected;
//     no code path may read organization_id from the request body (BOLA,
//     OWASP API1, TECH_SPEC §12).
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims are the identity facts extracted from a verified token.
type Claims struct {
	Subject string
	Email   string
}

// Authenticator verifies a raw bearer token.
type Authenticator interface {
	Verify(ctx context.Context, rawToken string) (Claims, error)
}

// JWK is the subset of a JSON Web Key this service understands (RSA only —
// the OIDC provider is configured to sign with RS256).
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksDocument struct {
	Keys []JWK `json:"keys"`
}

// JWKS fetches and caches the provider's signing keys. A key set refresh is
// triggered when a token references an unknown kid, so provider key rotation
// takes effect without a restart; a failed refresh keeps the previous set.
type JWKS struct {
	url    string
	client *http.Client
	now    func() time.Time

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey // kid -> key
	fetchedAt time.Time
}

const jwksCacheTTL = 15 * time.Minute

func NewJWKS(url string, client *http.Client) *JWKS {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &JWKS{url: url, client: client, now: time.Now, keys: map[string]*rsa.PublicKey{}}
}

// PublicKey resolves the signing key for kid, refreshing the cache when
// needed. Unknown kid after a forced refresh means the token is not ours.
func (j *JWKS) PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	j.mu.RLock()
	key, ok := j.keys[kid]
	fresh := j.now().Sub(j.fetchedAt) < jwksCacheTTL
	j.mu.RUnlock()

	if ok && fresh {
		return key, nil
	}
	if err := j.refresh(ctx); err != nil {
		if ok {
			// Stale keys beat no keys: verification continues with the last
			// known set instead of failing every request on a provider blip.
			return key, nil
		}
		return nil, fmt.Errorf("jwks refresh failed and kid %q unknown: %w", kid, err)
	}

	j.mu.RLock()
	defer j.mu.RUnlock()
	key, ok = j.keys[kid]
	if !ok {
		return nil, fmt.Errorf("kid %q not present in JWKS", kid)
	}
	return key, nil
}

func (j *JWKS) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return err
	}

	parsed := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pub, err := parseRSA(k)
		if err != nil {
			continue // skip malformed keys; others may still verify
		}
		parsed[k.Kid] = pub
	}
	if len(parsed) == 0 {
		return errors.New("jwks contained no usable RSA signing keys")
	}

	j.mu.Lock()
	j.keys = parsed
	j.fetchedAt = j.now()
	j.mu.Unlock()
	return nil
}

func parseRSA(k JWK) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("modulus: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("exponent: %w", err)
	}
	if len(eb) > 4 {
		return nil, errors.New("exponent too large")
	}
	e := 0
	for _, b := range eb {
		e = e<<8 | int(b)
	}
	if e == 0 {
		return nil, errors.New("empty exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
}

// OIDCAuthenticator verifies RS256 tokens against issuer/audience and the
// JWKS key set (TECH_SPEC §12: issuer, audience, expiry, signature).
type OIDCAuthenticator struct {
	issuer   string
	audience string
	jwks     *JWKS
}

func NewOIDCAuthenticator(issuer, audience, jwksURL string, client *http.Client) *OIDCAuthenticator {
	return &OIDCAuthenticator{issuer: issuer, audience: audience, jwks: NewJWKS(jwksURL, client)}
}

func (a *OIDCAuthenticator) Verify(ctx context.Context, rawToken string) (Claims, error) {
	keyFn := func(token *jwt.Token) (any, error) {
		// Algorithm pinning: the expected algorithm is part of the contract.
		// Accepting whatever the header claims is the classic "alg none" /
		// RS->HS confusion vulnerability (authz code must not be clever here).
		if token.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing algorithm %q, want RS256", token.Method.Alg())
		}
		kid, err := tokenKeyID(rawToken)
		if err != nil {
			return nil, err
		}
		// golang-jwt v5's keyfunc is not context-aware; the JWKS client's own
		// 5s timeout bounds the fetch instead of the request lifetime.
		return a.jwks.PublicKey(context.Background(), kid)
	}

	parsed, err := jwt.ParseWithClaims(rawToken, jwt.MapClaims{}, keyFn,
		jwt.WithIssuer(a.issuer),
		jwt.WithAudience(a.audience),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("token rejected: %w", err)
	}
	return claimsFromToken(parsed)
}

// claimsFromToken narrows generic claims into the identity facts this service
// relies on; missing mandatory claims invalidate the token outright.
func claimsFromToken(parsed *jwt.Token) (Claims, error) {
	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("token claims invalid")
	}
	sub, _ := mapClaims["sub"].(string)
	email, _ := mapClaims["email"].(string)
	if sub == "" || email == "" {
		return Claims{}, errors.New("token missing sub or email claim")
	}
	return Claims{Subject: sub, Email: email}, nil
}

// tokenKeyID extracts the kid header without verifying the signature — the
// value is only used to SELECT a trusted key, never to trust the token.
func tokenKeyID(rawToken string) (string, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("token header: %w", err)
	}
	var header struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", fmt.Errorf("token header: %w", err)
	}
	if header.Kid == "" {
		return "", errors.New("token header has no kid")
	}
	return header.Kid, nil
}
