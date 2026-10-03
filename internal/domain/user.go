// Package domain holds the core business types shared across layers. It has no
// dependencies on transport, storage or vendor concerns.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is the aggregate root that a profile and its credentials belong to.
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Address is a postal address in the shape used throughout the system.
type Address struct {
	StreetAddress string `json:"street_address"`
	Locality      string `json:"locality"`
	Region        string `json:"region"`
	PostalCode    string `json:"postal_code"`
	Country       string `json:"country"`
}

// Profile is the personal data stored for a user (1:1 with User).
type Profile struct {
	UserID  uuid.UUID `json:"user_id"`
	Name    string    `json:"name"`
	Phone   string    `json:"phone"`
	Address Address   `json:"address"`
}

// CredentialMethod identifies how a credential authenticates its owner.
type CredentialMethod string

// Supported credential methods.
const (
	MethodPassword CredentialMethod = "password"
	MethodOAuth    CredentialMethod = "oauth"
	MethodPasskey  CredentialMethod = "passkey"
)

// Valid reports whether m is a supported credential method.
func (m CredentialMethod) Valid() bool {
	switch m {
	case MethodPassword, MethodOAuth, MethodPasskey:
		return true
	}
	return false
}

// Credential is one way a user can authenticate. A user may hold several.
// SecretHash is never serialised.
type Credential struct {
	ID         uuid.UUID        `json:"id"`
	UserID     uuid.UUID        `json:"user_id"`
	Method     CredentialMethod `json:"method"`
	Username   string           `json:"username"`
	SecretHash string           `json:"-"`
	CreatedAt  time.Time        `json:"created_at"`
	LastUsedAt *time.Time       `json:"last_used_at,omitempty"`
}

// SearchQuery filters profile searches. Empty fields are ignored; at least one
// of Name, Phone or Username must be set.
type SearchQuery struct {
	Name     string
	Phone    string
	Username string
	Limit    int
	Offset   int
}

// Search pagination bounds.
const (
	DefaultSearchLimit = 20
	MaxSearchLimit     = 100
)

// Normalize clamps pagination values to sane bounds.
func (q SearchQuery) Normalize() SearchQuery {
	if q.Limit <= 0 {
		q.Limit = DefaultSearchLimit
	}
	if q.Limit > MaxSearchLimit {
		q.Limit = MaxSearchLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	return q
}

// IsEmpty reports whether no filter has been supplied.
func (q SearchQuery) IsEmpty() bool {
	return q.Name == "" && q.Phone == "" && q.Username == ""
}
