// Package provider defines the contract for third-party identity providers and
// the shared plumbing (resilient HTTP, token caching) that vendor adapters use.
package provider

import (
	"context"
	"errors"

	"github.com/navid/rolodex/internal/domain"
)

// Normalised provider errors. Adapters must wrap vendor-specific failures in
// one of these so callers can react without knowing the vendor.
var (
	// ErrUnavailable means the vendor could not be reached or kept failing
	// after retries (timeouts, 5xx). Safe to retry later.
	ErrUnavailable = errors.New("provider unavailable")
	// ErrAuth means the vendor rejected our service credentials.
	ErrAuth = errors.New("provider authentication failed")
	// ErrBadResponse means the vendor answered with something we cannot parse.
	ErrBadResponse = errors.New("provider returned an invalid response")
)

// IdentityProvider looks up personal data at a third-party vendor. Each
// implementation hides the vendor's auth flow and wire format and returns the
// normalised domain.Identity. domain.ErrNotFound means the vendor has no match.
type IdentityProvider interface {
	Name() string
	Lookup(ctx context.Context, q domain.IdentityQuery) (*domain.Identity, error)
}

// NormalizePhoneOrRaw normalises a vendor phone to E.164 so it can be compared
// with local data, falling back to the raw value if it cannot be parsed.
func NormalizePhoneOrRaw(raw string) string {
	if p, err := domain.NormalizePhone(raw); err == nil {
		return p
	}
	return raw
}
