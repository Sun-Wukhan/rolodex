package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/provider"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func validInput() CreateUserInput {
	return CreateUserInput{
		Name: "  Ada Lovelace ", Phone: "416-555-0100", Username: "Ada",
		Password: "a-long-password", Address: domain.Address{Locality: "Toronto", Country: "ca"},
	}
}

func TestCreateUserNormalisesAndHashes(t *testing.T) {
	repo := newMemRepo()
	svc := NewProfileService(repo, plainHasher{})
	u, err := svc.CreateUser(context.Background(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	p := repo.profiles[u.ID]
	if p.Name != "Ada Lovelace" || p.Phone != "+14165550100" || p.Address.Country != "CA" {
		t.Fatalf("not normalised: %+v", p)
	}
	if c := repo.creds[0]; c.Username != "ada" || c.SecretHash != "h:a-long-password" {
		t.Fatalf("credential: %+v", c)
	}
}

func TestCreateUserValidation(t *testing.T) {
	svc := NewProfileService(newMemRepo(), plainHasher{})
	in := CreateUserInput{Name: "", Phone: "12", Username: "a!", Password: "short", Address: domain.Address{Country: "Canada"}}
	_, err := svc.CreateUser(context.Background(), in)
	var ve *ValidationError
	if !errors.As(err, &ve) || !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	for _, f := range []string{"name", "phone", "username", "password", "address.country"} {
		if _, ok := ve.Fields[f]; !ok {
			t.Errorf("missing error for %s", f)
		}
	}
}

func TestSearchRequiresFilterAndNormalisesPhone(t *testing.T) {
	repo := newMemRepo()
	svc := NewProfileService(repo, plainHasher{})
	if _, err := svc.CreateUser(context.Background(), validInput()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Search(context.Background(), domain.SearchQuery{}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty search should fail, got %v", err)
	}
	got, err := svc.Search(context.Background(), domain.SearchQuery{Phone: "(416) 555-0100"})
	if err != nil || len(got) != 1 {
		t.Fatalf("phone search: %v %v", got, err)
	}
	if _, err := svc.Search(context.Background(), domain.SearchQuery{Phone: "abc"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("bad phone should fail, got %v", err)
	}
}

func TestGetUser(t *testing.T) {
	repo := newMemRepo()
	svc := NewProfileService(repo, plainHasher{})
	u, _ := svc.CreateUser(context.Background(), validInput())
	d, err := svc.GetUser(context.Background(), u.ID)
	if err != nil || len(d.Credentials) != 1 {
		t.Fatalf("GetUser: %+v %v", d, err)
	}
	if _, err := svc.GetUser(context.Background(), uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestLogin(t *testing.T) {
	repo := newMemRepo()
	ps := NewProfileService(repo, plainHasher{})
	u, _ := ps.CreateUser(context.Background(), validInput())
	auth, err := NewAuthService(repo, plainHasher{}, fakeIssuer{}, discard)
	if err != nil {
		t.Fatal(err)
	}

	tok, err := auth.Login(context.Background(), " ADA ", "a-long-password")
	if err != nil || tok.AccessToken != "token-for-"+u.ID.String() || tok.TokenType != "Bearer" {
		t.Fatalf("login: %+v %v", tok, err)
	}
	if repo.touched != 1 {
		t.Fatal("credential use not recorded")
	}

	for _, tc := range []struct{ user, pass string }{
		{"ada", "wrong-password"}, {"nobody", "a-long-password"}, {"", ""},
	} {
		if _, err := auth.Login(context.Background(), tc.user, tc.pass); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("Login(%q,%q) = %v, want ErrUnauthorized", tc.user, tc.pass, err)
		}
	}

	repo.failWith = errors.New("db down")
	if _, err := auth.Login(context.Background(), "ada", "a-long-password"); err == nil || errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("infrastructure errors must not look like bad credentials: %v", err)
	}
}

func TestEnrichMergesAndReportsPartialFailure(t *testing.T) {
	repo := newMemRepo()
	ps := NewProfileService(repo, plainHasher{})
	in := validInput()
	in.Address = domain.Address{Locality: "Toronto"}
	u, _ := ps.CreateUser(context.Background(), in)

	abc := &fakeProvider{name: "abc", id: &domain.Identity{
		Provider: "abc", Name: "ada lovelace", Phone: "+14165550100",
		Address: domain.Address{StreetAddress: "1 Main St", Locality: "Toronto", PostalCode: "M1M 1M1"},
	}}
	xyc := &fakeProvider{name: "xyc", err: provider.ErrUnavailable}
	slow := &fakeProvider{name: "slow", delay: time.Second}
	svc := NewIdentityService(repo, []provider.IdentityProvider{abc, xyc, slow}, 50*time.Millisecond, discard)

	res, err := svc.Enrich(context.Background(), u.ID, []string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, r := range res.Results {
		status[r.Provider] = r.Status
	}
	if status["abc"] != "ok" || status["xyc"] != "unavailable" || status["slow"] != "unavailable" {
		t.Fatalf("statuses: %v", status)
	}
	if f := res.Fields["name"]; f.Source != "local" || len(f.VerifiedBy) != 1 || f.VerifiedBy[0] != "abc" {
		t.Errorf("name should be local and verified by abc: %+v", f)
	}
	if f := res.Fields["street_address"]; f.Source != "abc" || f.Value != "1 Main St" {
		t.Errorf("street_address should be filled by abc: %+v", f)
	}
	if f := res.Fields["country"]; f.Value != "" || f.Source != "" {
		t.Errorf("country should stay empty: %+v", f)
	}
}

func TestEnrichErrors(t *testing.T) {
	repo := newMemRepo()
	svc := NewIdentityService(repo, []provider.IdentityProvider{&fakeProvider{name: "abc", err: domain.ErrNotFound}}, time.Second, discard)
	if _, err := svc.Enrich(context.Background(), uuid.New(), []string{"nope"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("unknown provider: %v", err)
	}
	if _, err := svc.Enrich(context.Background(), uuid.New(), nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	u, _ := NewProfileService(repo, plainHasher{}).CreateUser(context.Background(), validInput())
	res, err := svc.Enrich(context.Background(), u.ID, []string{"ABC", "abc"})
	if err != nil || len(res.Results) != 1 || res.Results[0].Status != "not_found" {
		t.Fatalf("not found mapping: %+v %v", res, err)
	}
	if got := svc.ProviderNames(); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("ProviderNames: %v", got)
	}
}
