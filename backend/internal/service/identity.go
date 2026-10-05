package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/provider"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
)

// IdentityService enriches local profiles with data from third-party identity
// providers.
type IdentityService struct {
	repo      repository.UserRepository
	providers map[string]provider.IdentityProvider
	timeout   time.Duration
	log       *slog.Logger
}

// NewIdentityService wires an IdentityService. timeout bounds each provider
// lookup (including retries).
func NewIdentityService(repo repository.UserRepository, providers []provider.IdentityProvider, timeout time.Duration, log *slog.Logger) *IdentityService {
	m := make(map[string]provider.IdentityProvider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &IdentityService{repo: repo, providers: m, timeout: timeout, log: log}
}

// ProviderNames lists the configured providers in sorted order.
func (s *IdentityService) ProviderNames() []string {
	names := make([]string, 0, len(s.providers))
	for n := range s.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Enrich queries the requested providers concurrently for the user's identity
// and merges the results with the local profile. A failing provider never
// fails the whole request: its status is reported in the result instead.
func (s *IdentityService) Enrich(ctx context.Context, userID uuid.UUID, names []string) (*domain.EnrichedProfile, error) {
	selected, err := s.selectProviders(names)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	query := domain.IdentityQuery{Name: profile.Name, Phone: profile.Phone}
	results := make([]domain.ProviderResult, len(selected))
	var wg sync.WaitGroup
	for i, p := range selected {
		wg.Add(1)
		go func(i int, p provider.IdentityProvider) {
			defer wg.Done()
			results[i] = s.lookup(ctx, p, query)
		}(i, p)
	}
	wg.Wait()

	return &domain.EnrichedProfile{
		UserID:  userID.String(),
		Fields:  merge(*profile, results),
		Results: results,
	}, nil
}

func (s *IdentityService) selectProviders(names []string) ([]provider.IdentityProvider, error) {
	if len(names) == 0 || (len(names) == 1 && names[0] == "all") {
		names = s.ProviderNames()
	}
	out := make([]provider.IdentityProvider, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if seen[n] {
			continue
		}
		p, ok := s.providers[n]
		if !ok {
			return nil, &ValidationError{Fields: map[string]string{"provider": fmt.Sprintf("unknown provider %q", n)}}
		}
		seen[n] = true
		out = append(out, p)
	}
	return out, nil
}

func (s *IdentityService) lookup(ctx context.Context, p provider.IdentityProvider, q domain.IdentityQuery) domain.ProviderResult {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	start := time.Now()
	id, err := p.Lookup(ctx, q)
	res := domain.ProviderResult{Provider: p.Name()}
	switch {
	case err == nil:
		res.Status, res.Identity = domain.ProviderStatusOK, id
	case errors.Is(err, domain.ErrNotFound):
		res.Status = domain.ProviderStatusNotFound
	case errors.Is(err, provider.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		res.Status, res.Error = domain.ProviderStatusUnavailable, "provider temporarily unavailable"
	default:
		res.Status, res.Error = domain.ProviderStatusError, "provider lookup failed"
	}
	// Only the error is logged, never the query or returned PII.
	s.log.InfoContext(ctx, "provider lookup", "provider", p.Name(), "status", res.Status,
		"duration_ms", time.Since(start).Milliseconds(), "error", errString(err))
	return res
}

// merge treats the local profile as the source of truth. Providers fill fields
// that are empty locally, and providers that return the same value as an
// existing field are recorded as having verified it.
func merge(p domain.Profile, results []domain.ProviderResult) map[string]domain.FieldSource {
	fields := map[string]domain.FieldSource{}
	set := func(key, value string) {
		f := domain.FieldSource{Value: value, VerifiedBy: []string{}}
		if value != "" {
			f.Source = domain.SourceLocal
		}
		fields[key] = f
	}
	set("name", p.Name)
	set("phone", p.Phone)
	set("street_address", p.Address.StreetAddress)
	set("locality", p.Address.Locality)
	set("region", p.Address.Region)
	set("postal_code", p.Address.PostalCode)
	set("country", p.Address.Country)

	for _, r := range results {
		if r.Identity == nil {
			continue
		}
		id := r.Identity
		for key, v := range map[string]string{
			"name": id.Name, "phone": id.Phone, "street_address": id.Address.StreetAddress,
			"locality": id.Address.Locality, "region": id.Address.Region,
			"postal_code": id.Address.PostalCode, "country": id.Address.Country,
		} {
			if v == "" {
				continue
			}
			f := fields[key]
			switch {
			case f.Value == "":
				fields[key] = domain.FieldSource{Value: v, Source: r.Provider, VerifiedBy: []string{}}
			case equalFold(f.Value, v):
				f.VerifiedBy = append(f.VerifiedBy, r.Provider)
				fields[key] = f
			}
		}
	}
	return fields
}

func equalFold(a, b string) bool {
	norm := func(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), "") }
	return norm(a) == norm(b)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
