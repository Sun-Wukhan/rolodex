// Package factory selects a repository implementation from configuration. It
// is the only place that knows about every concrete datastore.
package factory

import (
	"context"
	"fmt"

	"github.com/navid/rolodex/internal/repository"
	"github.com/navid/rolodex/internal/repository/postgres"
	"github.com/navid/rolodex/internal/repository/sqlite"
)

// Supported drivers.
const (
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

// Open returns a UserRepository for driver ("postgres" or "sqlite") using dsn.
func Open(ctx context.Context, driver, dsn string) (repository.UserRepository, error) {
	switch driver {
	case DriverPostgres:
		return postgres.Open(ctx, dsn)
	case DriverSQLite:
		return sqlite.Open(ctx, dsn)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q (want %q or %q)", driver, DriverPostgres, DriverSQLite)
	}
}
