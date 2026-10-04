// Package repository defines the persistence contract used by the service
// layer. Concrete implementations live in sub-packages (postgres, sqlite) and
// are selected at startup; nothing above this layer knows which database is in
// use.
//
// Profiles and credentials are kept in two separate databases so that the one
// holding password hashes can be isolated, access-controlled and backed up on
// its own. ProfileStore and CredentialStore are the per-database contracts;
// the split package combines them into a UserRepository.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
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

// ProfileStore persists users and their profiles (the profiles database).
type ProfileStore interface {
	// CreateUser inserts a user and its profile atomically, using u.ID as the
	// key. It returns domain.ErrConflict if the ID already exists.
	CreateUser(ctx context.Context, u domain.User, p domain.Profile) error
	// DeleteUser removes a user and its profile. Deleting an unknown user is
	// not an error.
	DeleteUser(ctx context.Context, userID uuid.UUID) error
	// GetProfile returns the profile for a user or domain.ErrNotFound.
	GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error)
	// SearchProfiles applies the name and phone filters, limit and offset of
	// q (q.Username is ignored). A non-nil userIDs restricts results to those
	// users.
	SearchProfiles(ctx context.Context, q domain.SearchQuery, userIDs []uuid.UUID) ([]domain.Profile, error)
	// Ping verifies the datastore is reachable.
	Ping(ctx context.Context) error
	// Close releases underlying resources.
	Close() error
}

// CredentialStore persists login credentials and secret hashes (the
// credentials database). It does not verify that UserID refers to an existing
// user; that is the caller's responsibility.
type CredentialStore interface {
	// AddCredential inserts a credential, assigning its ID and creation time.
	// It returns domain.ErrConflict if (method, username) is already taken.
	AddCredential(ctx context.Context, c domain.Credential) (domain.Credential, error)
	// DeleteCredentials removes every credential owned by a user.
	DeleteCredentials(ctx context.Context, userID uuid.UUID) error
	// GetCredential looks up a credential by method and username (case-insensitive).
	GetCredential(ctx context.Context, method domain.CredentialMethod, username string) (*domain.Credential, error)
	// ListCredentials returns all credentials for a user.
	ListCredentials(ctx context.Context, userID uuid.UUID) ([]domain.Credential, error)
	// TouchCredential records a successful use of a credential.
	TouchCredential(ctx context.Context, credentialID uuid.UUID) error
	// UserIDsByUsername returns the distinct IDs of users owning a credential
	// whose username contains substr, case-insensitively.
	UserIDsByUsername(ctx context.Context, substr string) ([]uuid.UUID, error)
	// Ping verifies the datastore is reachable.
	Ping(ctx context.Context) error
	// Close releases underlying resources.
	Close() error
}
