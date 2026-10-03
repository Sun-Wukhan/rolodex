// Package repository defines the persistence contract used by the service
// layer. Concrete implementations live in sub-packages (postgres, sqlite) and
// are selected at startup; nothing above this layer knows which database is in
// use.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/navid/rolodex/internal/domain"
)

// UserRepository stores and retrieves users, their profile and credentials.
//
// Implementations must translate driver errors into domain errors
// (domain.ErrNotFound, domain.ErrConflict) and must be safe for concurrent use.
type UserRepository interface {
	// CreateUser atomically creates a user with its profile and first
	// credential. The IDs on p and c are ignored and assigned by the repository.
	CreateUser(ctx context.Context, p domain.Profile, c domain.Credential) (domain.User, error)
	// GetProfile returns the profile for a user or domain.ErrNotFound.
	GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error)
	// SearchProfiles returns profiles matching all non-empty filters in q.
	// Name and username match case-insensitively by substring; phone matches
	// exactly on the normalised value.
	SearchProfiles(ctx context.Context, q domain.SearchQuery) ([]domain.Profile, error)
	// AddCredential attaches an additional credential to an existing user.
	AddCredential(ctx context.Context, c domain.Credential) (domain.Credential, error)
	// GetCredential looks up a credential by method and username (case-insensitive).
	GetCredential(ctx context.Context, method domain.CredentialMethod, username string) (*domain.Credential, error)
	// ListCredentials returns all credentials for a user.
	ListCredentials(ctx context.Context, userID uuid.UUID) ([]domain.Credential, error)
	// TouchCredential records a successful use of a credential.
	TouchCredential(ctx context.Context, credentialID uuid.UUID) error
	// Ping verifies the datastore is reachable.
	Ping(ctx context.Context) error
	// Close releases underlying resources.
	Close() error
}
