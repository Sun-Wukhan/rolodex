// Package migrations embeds the per-dialect, per-database SQL migrations and
// applies them with goose.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed postgres/*/*.sql sqlite/*/*.sql
var files embed.FS

// Schema identifies which of the two databases a migration set belongs to.
type Schema string

// The profiles and credentials databases are migrated independently.
const (
	// SchemaProfiles holds users and their profile data.
	SchemaProfiles Schema = "profiles"
	// SchemaCredentials holds login credentials and password hashes.
	SchemaCredentials Schema = "credentials"
)

// versionTables keeps each schema's goose bookkeeping separate. Profiles keeps
// goose's default table so databases created before the split upgrade in place.
var versionTables = map[Schema]string{ //nolint:gosec // G101: table names, not secrets
	SchemaProfiles:    "goose_db_version",
	SchemaCredentials: "goose_credentials_db_version",
}

// Up applies all pending migrations of schema for the given dialect
// ("postgres" or "sqlite").
func Up(ctx context.Context, db *sql.DB, dialect string, schema Schema) error {
	table, ok := versionTables[schema]
	if !ok {
		return fmt.Errorf("migrations: unknown schema %q", schema)
	}
	var gd goose.Dialect
	opts := []goose.ProviderOption{goose.WithTableName(table)}
	switch dialect {
	case "postgres":
		gd = goose.DialectPostgres
		// Replicas start concurrently (e.g. a Kubernetes Deployment); an
		// advisory lock ensures only one of them applies migrations.
		locker, err := lock.NewPostgresSessionLocker()
		if err != nil {
			return fmt.Errorf("migrations: %w", err)
		}
		opts = append(opts, goose.WithSessionLocker(locker))
	case "sqlite":
		gd = goose.DialectSQLite3
	default:
		return fmt.Errorf("migrations: unsupported dialect %q", dialect)
	}
	sub, err := fs.Sub(files, path.Join(dialect, string(schema)))
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	provider, err := goose.NewProvider(gd, db, sub, opts...)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrations: up %s: %w", schema, err)
	}
	return nil
}
