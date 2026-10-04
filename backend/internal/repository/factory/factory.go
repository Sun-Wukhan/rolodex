// Package factory selects a repository implementation from configuration. It
// is the only place that knows about every concrete datastore.
package factory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/internal/repository/postgres"
	"github.com/Sun-Wukhan/rolodex/internal/repository/replica"
	"github.com/Sun-Wukhan/rolodex/internal/repository/split"
	"github.com/Sun-Wukhan/rolodex/internal/repository/sqlite"
)

// Supported drivers.
const (
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

type options struct {
	profilesReplicaDSN string
	log                *slog.Logger
}

// Option customises Open.
type Option func(*options)

// WithProfilesReplica sends profile reads to the read-only replica at dsn,
// keeping writes on the primary. An empty dsn disables it. PostgreSQL only.
func WithProfilesReplica(dsn string, log *slog.Logger) Option {
	return func(o *options) {
		o.profilesReplicaDSN = dsn
		o.log = log
	}
}

// Open returns a UserRepository for driver ("postgres" or "sqlite") that keeps
// profiles in the database at profilesDSN and credentials in the separate
// database at credentialsDSN.
func Open(ctx context.Context, driver, profilesDSN, credentialsDSN string, opts ...Option) (repository.UserRepository, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if profilesDSN == "" || credentialsDSN == "" {
		return nil, errors.New("both a profiles and a credentials database must be configured")
	}
	if profilesDSN == credentialsDSN {
		return nil, errors.New("profiles and credentials must use different databases")
	}
	switch driver {
	case DriverPostgres:
		if o.profilesReplicaDSN != "" {
			if o.profilesReplicaDSN == profilesDSN {
				return nil, errors.New("the profiles replica must be a different server than the primary")
			}
			return openPair(ctx, profilesDSN, credentialsDSN, withReplica(o), postgres.OpenCredentials)
		}
		return openPair(ctx, profilesDSN, credentialsDSN, postgres.OpenProfiles, postgres.OpenCredentials)
	case DriverSQLite:
		if o.profilesReplicaDSN != "" {
			return nil, errors.New("a profiles read replica requires DB_DRIVER=postgres")
		}
		return openPair(ctx, profilesDSN, credentialsDSN, sqlite.OpenProfiles, sqlite.OpenCredentials)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q (want %q or %q)", driver, DriverPostgres, DriverSQLite)
	}
}

// withReplica returns an opener for a primary/replica profile store pair.
func withReplica(o options) func(context.Context, string) (repository.ProfileStore, error) {
	return func(ctx context.Context, primaryDSN string) (repository.ProfileStore, error) {
		primary, err := postgres.OpenProfiles(ctx, primaryDSN)
		if err != nil {
			return nil, err
		}
		rep, err := postgres.OpenProfilesReplica(o.profilesReplicaDSN)
		if err != nil {
			_ = primary.Close()
			return nil, err
		}
		log := o.log
		if log == nil {
			log = slog.Default()
		}
		return replica.New(primary, rep, log), nil
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
