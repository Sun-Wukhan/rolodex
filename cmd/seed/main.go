// Command seed loads demo users through the ProfileService so they are
// validated and hashed exactly like API-created users. It is idempotent:
// existing usernames are skipped.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/internal/repository/factory"
	"github.com/navid/rolodex/internal/security"
	"github.com/navid/rolodex/internal/service"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	password := os.Getenv("SEED_PASSWORD")
	if len(password) < 12 {
		return errors.New("SEED_PASSWORD must be set (at least 12 characters)")
	}
	driver := getenv("DB_DRIVER", "sqlite")
	dsn := getenv("DATABASE_URL", "rolodex.db")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	repo, err := factory.Open(ctx, driver, dsn)
	if err != nil {
		return err
	}
	defer func() { _ = repo.Close() }()

	svc := service.NewProfileService(repo, security.NewArgon2Hasher(security.DefaultArgon2Params()))
	for _, u := range demoUsers(password) {
		_, err := svc.CreateUser(ctx, u)
		switch {
		case errors.Is(err, domain.ErrConflict):
			log.Info("user exists, skipping", "username", u.Username)
		case err != nil:
			return err
		default:
			log.Info("user created", "username", u.Username)
		}
	}
	return nil
}

// demoUsers line up with the mock vendor datasets so enrichment demonstrates
// verified fields, filled gaps, mismatches and not-found results.
func demoUsers(password string) []service.CreateUserInput {
	return []service.CreateUserInput{
		{Name: "Rolodex Admin", Phone: "416-555-0100", Username: "admin", Password: password,
			Address: domain.Address{Locality: "Toronto", Region: "ON", Country: "CA"}},
		{Name: "Ada Lovelace", Phone: "416-555-0101", Username: "ada", Password: password,
			Address: domain.Address{StreetAddress: "100 King St W", Locality: "Toronto", Region: "ON", PostalCode: "M5X 1A9", Country: "CA"}},
		{Name: "Grace Hopper", Phone: "212-555-0102", Username: "grace", Password: password,
			Address: domain.Address{Locality: "New York", Region: "NY", Country: "US"}},
		{Name: "Alan Turing", Phone: "+44 20 7946 0103", Username: "alan", Password: password},
		{Name: "Katherine Johnson", Phone: "757-555-0104", Username: "katherine", Password: password,
			Address: domain.Address{StreetAddress: "1 NASA Dr", Locality: "Hampton", Region: "VA", PostalCode: "23681", Country: "US"}},
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
