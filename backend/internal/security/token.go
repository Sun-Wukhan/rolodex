package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for any token that fails verification.
var ErrInvalidToken = errors.New("invalid token")

// Claims are the JWT claims issued by Rolodex.
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies HS256 access tokens.
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenManager creates a TokenManager. secret must be at least 32 bytes.
func NewTokenManager(secret []byte, issuer string, ttl time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("security: JWT secret must be at least 32 bytes")
	}
	return &TokenManager{secret: secret, issuer: issuer, ttl: ttl, now: time.Now}, nil
}

// TTL returns the access token lifetime.
func (m *TokenManager) TTL() time.Duration { return m.ttl }

// Issue signs a token for the given subject (user ID) and username.
func (m *TokenManager) Issue(subject, username string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("security: sign: %w", err)
	}
	return signed, exp, nil
}

// Verify parses and validates a token, pinning the algorithm to HS256 to
// prevent alg-confusion attacks.
func (m *TokenManager) Verify(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
