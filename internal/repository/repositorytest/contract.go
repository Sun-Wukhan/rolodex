// Package repositorytest provides a contract test suite that every
// repository.UserRepository implementation must pass. Running the same suite
// against each database proves the implementations are interchangeable.
package repositorytest

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
)

// Factory returns a fresh, empty repository for a single test.
type Factory func(t *testing.T) repository.UserRepository

// Run executes the full contract suite against repositories built by newRepo.
func Run(t *testing.T, newRepo Factory) {
	t.Helper()
	tests := map[string]func(*testing.T, repository.UserRepository){
		"CreateAndGetProfile":        testCreateAndGetProfile,
		"GetProfileNotFound":         testGetProfileNotFound,
		"DuplicateUsernameConflicts": testDuplicateUsername,
		"SearchByNamePhoneUsername":  testSearch,
		"SearchPaginationAndEscape":  testSearchPaginationAndEscape,
		"MultipleCredentials":        testMultipleCredentials,
		"AddCredentialUnknownUser":   testAddCredentialUnknownUser,
		"TouchCredential":            testTouchCredential,
	}
	for name, fn := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newRepo(t)
			fn(t, repo)
		})
	}
}

func mustCreate(t *testing.T, r repository.UserRepository, name, phone, username string) domain.User {
	t.Helper()
	u, err := r.CreateUser(context.Background(),
		domain.Profile{Name: name, Phone: phone, Address: domain.Address{
			StreetAddress: "1 Main St", Locality: "Toronto", Region: "ON", PostalCode: "M1M1M1", Country: "CA",
		}},
		domain.Credential{Method: domain.MethodPassword, Username: username, SecretHash: "hash-" + username})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", username, err)
	}
	return u
}

func testCreateAndGetProfile(t *testing.T, r repository.UserRepository) {
	u := mustCreate(t, r, "Ada Lovelace", "+14165550001", "ada")
	p, err := r.GetProfile(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if p.UserID != u.ID || p.Name != "Ada Lovelace" || p.Address.Locality != "Toronto" {
		t.Fatalf("unexpected profile %+v", p)
	}
}

func testGetProfileNotFound(t *testing.T, r repository.UserRepository) {
	_, err := r.GetProfile(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	_, err = r.GetCredential(context.Background(), domain.MethodPassword, "nobody")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func testDuplicateUsername(t *testing.T, r repository.UserRepository) {
	mustCreate(t, r, "First", "+14165550002", "dup")
	_, err := r.CreateUser(context.Background(),
		domain.Profile{Name: "Second", Phone: "+14165550003"},
		domain.Credential{Method: domain.MethodPassword, Username: "dup", SecretHash: "x"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	// The failed transaction must not leave a half-created profile behind.
	got, err := r.SearchProfiles(context.Background(), domain.SearchQuery{Name: "Second"})
	if err != nil || len(got) != 0 {
		t.Fatalf("rollback failed: %v %v", got, err)
	}
}

func testSearch(t *testing.T, r repository.UserRepository) {
	ctx := context.Background()
	mustCreate(t, r, "Grace Hopper", "+14165550010", "grace")
	mustCreate(t, r, "Grace Kelly", "+14165550011", "gkelly")
	mustCreate(t, r, "Alan Turing", "+14165550012", "alan")

	cases := []struct {
		name string
		q    domain.SearchQuery
		want int
	}{
		{"name substring case-insensitive", domain.SearchQuery{Name: "grace"}, 2},
		{"phone exact", domain.SearchQuery{Phone: "+14165550012"}, 1},
		{"username substring", domain.SearchQuery{Username: "KELL"}, 1},
		{"combined filters AND", domain.SearchQuery{Name: "grace", Username: "grace"}, 1},
		{"no match", domain.SearchQuery{Name: "zzz"}, 0},
	}
	for _, c := range cases {
		got, err := r.SearchProfiles(ctx, c.q)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(got) != c.want {
			t.Errorf("%s: got %d results, want %d", c.name, len(got), c.want)
		}
	}
}

func testSearchPaginationAndEscape(t *testing.T, r repository.UserRepository) {
	ctx := context.Background()
	mustCreate(t, r, "User A", "+14165550020", "ua")
	mustCreate(t, r, "User B", "+14165550021", "ub")
	mustCreate(t, r, "User C", "+14165550022", "uc")

	page1, err := r.SearchProfiles(ctx, domain.SearchQuery{Name: "user", Limit: 2})
	if err != nil || len(page1) != 2 {
		t.Fatalf("page1: %v %v", page1, err)
	}
	page2, err := r.SearchProfiles(ctx, domain.SearchQuery{Name: "user", Limit: 2, Offset: 2})
	if err != nil || len(page2) != 1 || page2[0].Name != "User C" {
		t.Fatalf("page2: %v %v", page2, err)
	}
	// LIKE wildcards in user input must be treated literally.
	got, err := r.SearchProfiles(ctx, domain.SearchQuery{Name: "%"})
	if err != nil || len(got) != 0 {
		t.Fatalf("wildcard not escaped: %v %v", got, err)
	}
}

func testMultipleCredentials(t *testing.T, r repository.UserRepository) {
	ctx := context.Background()
	u := mustCreate(t, r, "Multi", "+14165550030", "multi")
	if _, err := r.AddCredential(ctx, domain.Credential{UserID: u.ID, Method: domain.MethodOAuth, Username: "multi@example.com"}); err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	creds, err := r.ListCredentials(ctx, u.ID)
	if err != nil || len(creds) != 2 {
		t.Fatalf("ListCredentials: %v %v", creds, err)
	}
	c, err := r.GetCredential(ctx, domain.MethodPassword, "MULTI")
	if err != nil || c.UserID != u.ID || c.SecretHash != "hash-multi" {
		t.Fatalf("GetCredential: %+v %v", c, err)
	}
	oauth, err := r.GetCredential(ctx, domain.MethodOAuth, "multi@example.com")
	if err != nil || oauth.SecretHash != "" {
		t.Fatalf("oauth credential: %+v %v", oauth, err)
	}
}

func testAddCredentialUnknownUser(t *testing.T, r repository.UserRepository) {
	_, err := r.AddCredential(context.Background(), domain.Credential{UserID: uuid.New(), Method: domain.MethodPasskey, Username: "ghost"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func testTouchCredential(t *testing.T, r repository.UserRepository) {
	ctx := context.Background()
	mustCreate(t, r, "Touch", "+14165550040", "touch")
	c, err := r.GetCredential(ctx, domain.MethodPassword, "touch")
	if err != nil {
		t.Fatal(err)
	}
	if c.LastUsedAt != nil {
		t.Fatal("expected nil LastUsedAt before touch")
	}
	if err := r.TouchCredential(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = r.GetCredential(ctx, domain.MethodPassword, "touch")
	if c.LastUsedAt == nil {
		t.Fatal("expected LastUsedAt after touch")
	}
	if err := r.TouchCredential(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
