// Package security contains password hashing and token primitives.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrMismatch is returned when a password does not match its hash.
var ErrMismatch = errors.New("password mismatch")

// Argon2Params are the Argon2id cost parameters. Defaults follow the OWASP
// recommendation (m=19 MiB, t=2, p=1).
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Params returns production-grade parameters.
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) error
}

// Argon2Hasher implements PasswordHasher with Argon2id and PHC-format strings:
// $argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>
type Argon2Hasher struct {
	Params Argon2Params
}

// NewArgon2Hasher returns a hasher with the given parameters.
func NewArgon2Hasher(p Argon2Params) *Argon2Hasher { return &Argon2Hasher{Params: p} }

// Hash derives an encoded Argon2id hash with a random salt.
func (h *Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, h.Params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("security: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.Params.Iterations, h.Params.Memory, h.Params.Parallelism, h.Params.KeyLength)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.Params.Memory, h.Params.Iterations, h.Params.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify checks password against an encoded hash in constant time. Parameters
// are read from the encoded string so old hashes keep working after a cost
// change.
func (h *Argon2Hasher) Verify(password, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("security: unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return fmt.Errorf("security: unsupported argon2 version")
	}
	var p Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return fmt.Errorf("security: bad params: %w", err)
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("security: bad salt: %w", err)
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("security: bad hash: %w", err)
	}
	if len(want) < 16 || len(want) > 128 {
		return fmt.Errorf("security: unsupported key length")
	}
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, uint32(len(want))) //nolint:gosec // bounded to 128 above
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}
