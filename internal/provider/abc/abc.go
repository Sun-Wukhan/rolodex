// Package abc adapts the ABC identity vendor to provider.IdentityProvider.
// ABC's /identity response matches the exercise specification directly:
// {name, phone, address{street_address, locality, region, postal_code, country}}.
package abc

import (
	"context"

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/internal/provider"
)

// Name is the provider identifier used in API requests and provenance.
const Name = "abc"

type identityResponse struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address struct {
		StreetAddress string `json:"street_address"`
		Locality      string `json:"locality"`
		Region        string `json:"region"`
		PostalCode    string `json:"postal_code"`
		Country       string `json:"country"`
	} `json:"address"`
}

// Provider is the ABC adapter.
type Provider struct {
	client *provider.Client
}

// New creates an ABC provider using the shared resilient client.
func New(cfg provider.ClientConfig) *Provider {
	return &Provider{client: provider.NewClient(cfg)}
}

// Name returns "abc".
func (p *Provider) Name() string { return Name }

// Lookup queries ABC's /identity endpoint and maps the result.
func (p *Provider) Lookup(ctx context.Context, q domain.IdentityQuery) (*domain.Identity, error) {
	var resp identityResponse
	if err := p.client.PostJSON(ctx, "/identity", q, &resp); err != nil {
		return nil, err
	}
	return &domain.Identity{
		Provider: Name,
		Name:     resp.Name,
		Phone:    provider.NormalizePhoneOrRaw(resp.Phone),
		Address: domain.Address{
			StreetAddress: resp.Address.StreetAddress,
			Locality:      resp.Address.Locality,
			Region:        resp.Address.Region,
			PostalCode:    resp.Address.PostalCode,
			Country:       resp.Address.Country,
		},
	}, nil
}
