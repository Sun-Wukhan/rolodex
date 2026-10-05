package security

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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// FirebaseKeysURL publishes the keys that sign Firebase Authentication ID
// tokens, as a JWK set.
const FirebaseKeysURL = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

const (
	firebaseLeeway        = time.Minute
	defaultKeysMaxAge     = time.Hour
	minKeysRefreshBackoff = time.Minute
	maxKeysBodyBytes      = 1 << 20
)

// ErrKeysUnavailable is returned when the signing keys cannot be fetched. It
// is an infrastructure failure, not a sign that the token is invalid.
var ErrKeysUnavailable = errors.New("firebase signing keys unavailable")

// FirebaseIdentity is the verified subset of a Firebase ID token.
type FirebaseIdentity struct {
	UID           string
	Email         string
	EmailVerified bool
	Name          string
}

type firebaseClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	AuthTime      int64  `json:"auth_time"`
	jwt.RegisteredClaims
}

// FirebaseVerifier verifies Firebase Authentication ID tokens for one project
// against Google's published signing keys. It needs only the project ID: no
// service account or other secret is involved.
type FirebaseVerifier struct {
	projectID string
	keysURL   string
	client    *http.Client
	now       func() time.Time

	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	keysExpire  time.Time
	lastAttempt time.Time
}

// NewFirebaseVerifier creates a verifier for projectID. keysURL is normally
// FirebaseKeysURL; tests point it at a local server.
func NewFirebaseVerifier(projectID, keysURL string, client *http.Client) (*FirebaseVerifier, error) {
	if projectID == "" {
		return nil, errors.New("security: firebase project ID is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &FirebaseVerifier{projectID: projectID, keysURL: keysURL, client: client, now: time.Now}, nil
}

// Verify checks the token's signature, issuer, audience and lifetime and
// returns the identity it asserts. Any token problem yields ErrInvalidToken;
// a failure to obtain signing keys yields ErrKeysUnavailable.
func (v *FirebaseVerifier) Verify(ctx context.Context, token string) (FirebaseIdentity, error) {
	claims := &firebaseClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			if kid == "" {
				return nil, ErrInvalidToken
			}
			return v.key(ctx, kid)
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer("https://securetoken.google.com/"+v.projectID),
		jwt.WithAudience(v.projectID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(firebaseLeeway),
		jwt.WithTimeFunc(v.now),
	)
	if err != nil {
		if errors.Is(err, ErrKeysUnavailable) {
			return FirebaseIdentity{}, err
		}
		return FirebaseIdentity{}, ErrInvalidToken
	}
	if claims.Subject == "" || len(claims.Subject) > 128 {
		return FirebaseIdentity{}, ErrInvalidToken
	}
	if claims.AuthTime == 0 || time.Unix(claims.AuthTime, 0).After(v.now().Add(firebaseLeeway)) {
		return FirebaseIdentity{}, ErrInvalidToken
	}
	return FirebaseIdentity{
		UID: claims.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified, Name: claims.Name,
	}, nil
}

// key returns the public key for kid, refreshing the cached key set when it
// has expired or does not know kid (keys rotate). Refreshes triggered by
// unknown key IDs are rate limited so forged tokens cannot hammer Google.
func (v *FirebaseVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	if k, ok := v.keys[kid]; ok && now.Before(v.keysExpire) {
		return k, nil
	}
	if !v.lastAttempt.IsZero() && now.Sub(v.lastAttempt) < minKeysRefreshBackoff {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
		if v.keys == nil {
			return nil, ErrKeysUnavailable
		}
		return nil, ErrInvalidToken
	}
	v.lastAttempt = now
	keys, maxAge, err := v.fetchKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeysUnavailable, err)
	}
	v.keys, v.keysExpire = keys, now.Add(maxAge)
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, ErrInvalidToken
}

type jwkSet struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *FirebaseVerifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.keysURL, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var set jwkSet
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxKeysBodyBytes)).Decode(&set); err != nil {
		return nil, 0, fmt.Errorf("decode: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Alg != "" && k.Alg != "RS256") || k.Kid == "" {
			continue
		}
		pub, err := rsaKey(k.N, k.E)
		if err != nil {
			return nil, 0, fmt.Errorf("key %s: %w", k.Kid, err)
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, 0, errors.New("no RSA signing keys published")
	}
	return keys, maxAge(resp.Header.Get("Cache-Control")), nil
}

func rsaKey(n, e string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("modulus: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("exponent: %w", err)
	}
	exp := new(big.Int).SetBytes(eb)
	if !exp.IsInt64() || exp.Int64() < 3 || exp.Int64() > 1<<31-1 {
		return nil, errors.New("unsupported exponent")
	}
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(exp.Int64())}
	if pub.N.BitLen() < 2048 {
		return nil, errors.New("modulus shorter than 2048 bits")
	}
	return pub, nil
}

// maxAge reads max-age from a Cache-Control header, defaulting to an hour.
func maxAge(cacheControl string) time.Duration {
	for _, directive := range strings.Split(cacheControl, ",") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(directive), "max-age="); ok {
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return defaultKeysMaxAge
}
