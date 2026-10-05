// Package xyc adapts the XYC identity vendor to provider.IdentityProvider.
// XYC uses its own response shape, which this adapter normalises:
//
//	{"person": {"full_name": "...", "msisdn": "...",
//	  "addr": {"line1": "...", "city": "...", "state": "...", "zip": "...", "country_code": "..."}}}
package xyc

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/provider"
)

// Name is the provider identifier used in API requests and provenance.
const Name = "xyc"

type identityResponse struct {
	Person *struct {
		FullName string `json:"full_name"`
		MSISDN   string `json:"msisdn"`
		Addr     struct {
			Line1       string `json:"line1"`
			City        string `json:"city"`
			State       string `json:"state"`
			Zip         string `json:"zip"`
			CountryCode string `json:"country_code"`
		} `json:"addr"`
	} `json:"person"`
}

// Provider is the XYC adapter.
type Provider struct {
	client *provider.Client
}

// New creates an XYC provider using the shared resilient client.
func New(cfg provider.ClientConfig) *Provider {
	return &Provider{client: provider.NewClient(cfg)}
}

// Name returns "xyc".
func (p *Provider) Name() string { return Name }

// Lookup queries XYC's /identity endpoint and maps its schema to the domain.
func (p *Provider) Lookup(ctx context.Context, q domain.IdentityQuery) (*domain.Identity, error) {
	var resp identityResponse
	if err := p.client.PostJSON(ctx, "/identity", q, &resp); err != nil {
		return nil, err
	}
	if resp.Person == nil {
		return nil, fmt.Errorf("%w: missing person", provider.ErrBadResponse)
	}
	a := resp.Person.Addr
	return &domain.Identity{
		Provider: Name,
		Name:     resp.Person.FullName,
		Phone:    provider.NormalizePhoneOrRaw(resp.Person.MSISDN),
		Address: domain.Address{
			StreetAddress: a.Line1,
			Locality:      a.City,
			Region:        a.State,
			PostalCode:    a.Zip,
			Country:       strings.ToUpper(a.CountryCode),
		},
	}, nil
}
