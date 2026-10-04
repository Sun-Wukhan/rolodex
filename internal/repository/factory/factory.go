// Package factory selects a repository implementation from configuration. It
// is the only place that knows about every concrete datastore.
package factory

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/internal/repository/postgres"
	"github.com/Sun-Wukhan/rolodex/internal/repository/split"
	"github.com/Sun-Wukhan/rolodex/internal/repository/sqlite"
)

// Supported drivers.
const (
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

// Open returns a UserRepository for driver ("postgres" or "sqlite") that keeps
// profiles in the database at profilesDSN and credentials in the separate
// database at credentialsDSN.
func Open(ctx context.Context, driver, profilesDSN, credentialsDSN string) (repository.UserRepository, error) {
	if profilesDSN == "" || credentialsDSN == "" {
		return nil, errors.New("both a profiles and a credentials database must be configured")
	}
	if profilesDSN == credentialsDSN {
		return nil, errors.New("profiles and credentials must use different databases")
	}
	switch driver {
	case DriverPostgres:
		return openPair(ctx, profilesDSN, credentialsDSN, postgres.OpenProfiles, postgres.OpenCredentials)
	case DriverSQLite:
		return openPair(ctx, profilesDSN, credentialsDSN, sqlite.OpenProfiles, sqlite.OpenCredentials)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q (want %q or %q)", driver, DriverPostgres, DriverSQLite)
	}
}

// openPair opens both stores of one driver, closing the first if the second
// fails.
func openPair[P repository.ProfileStore, C repository.CredentialStore](
	ctx context.Context, profilesDSN, credentialsDSN string,
	openProfiles func(context.Context, string) (P, error),
	openCredentials func(context.Context, string) (C, error),
) (repository.UserRepository, error) {
	profiles, err := openProfiles(ctx, profilesDSN)
	if err != nil {
		return nil, err
	}
	credentials, err := openCredentials(ctx, credentialsDSN)
	if err != nil {
		_ = profiles.Close()
		return nil, err
	}
	return split.New(profiles, credentials), nil
}
