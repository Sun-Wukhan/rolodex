package factory_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository/factory"
)

func TestOpenRejectsSharedOrMissingDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "one.db")
	for name, dsns := range map[string][2]string{
		"same":    {path, path},
		"missing": {path, ""},
	} {
		if _, err := factory.Open(ctx, factory.DriverSQLite, dsns[0], dsns[1]); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := factory.Open(ctx, "mysql", "a", "b"); err == nil {
		t.Error("unknown driver: expected error")
	}
}

func TestOpenSQLiteKeepsCredentialsInSeparateFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	repo, err := factory.Open(ctx, factory.DriverSQLite, filepath.Join(dir, "profiles.db"), filepath.Join(dir, "credentials.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	u, err := repo.CreateUser(ctx, domain.Profile{Name: "Ada", Phone: "+14165550001"},
		domain.Credential{Method: domain.MethodPassword, Username: "ada", SecretHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.GetCredential(ctx, domain.MethodPassword, "ada")
	if err != nil || c.UserID != u.ID {
		t.Fatalf("credential: %+v %v", c, err)
	}
	if err := repo.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	for file, want := range map[string]map[string]bool{
		"profiles.db":    {"users": true, "user_profiles": true, "user_credentials": false},
		"credentials.db": {"users": false, "user_profiles": false, "user_credentials": true},
	} {
		tables := tablesIn(t, filepath.Join(dir, file))
		for table, present := range want {
			if tables[table] != present {
				t.Errorf("%s: table %s present=%v, want %v", file, table, tables[table], present)
			}
		}
	}
}

func tablesIn(t *testing.T, path string) map[string]bool {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
