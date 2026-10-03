package domain

// IdentityQuery is what we send to third-party identity providers.
type IdentityQuery struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Identity is the normalised PII returned by any identity provider, regardless
// of the vendor's wire format.
type Identity struct {
	Provider string  `json:"provider"`
	Name     string  `json:"name"`
	Phone    string  `json:"phone"`
	Address  Address `json:"address"`
}

// FieldSource records which source supplied a field in an enriched profile.
type FieldSource struct {
	Value  string `json:"value"`
	Source string `json:"source"`
}

// ProviderResult describes the outcome of a single provider lookup.
type ProviderResult struct {
	Provider string    `json:"provider"`
	Status   string    `json:"status"`
	Error    string    `json:"error,omitempty"`
	Identity *Identity `json:"identity,omitempty"`
}

// Provider lookup statuses.
const (
	ProviderStatusOK          = "ok"
	ProviderStatusNotFound    = "not_found"
	ProviderStatusUnavailable = "unavailable"
	ProviderStatusError       = "error"
)

// EnrichedProfile merges the locally stored profile with provider data, keeping
// per-field provenance so consumers can see where each value came from.
type EnrichedProfile struct {
	UserID  string                 `json:"user_id"`
	Fields  map[string]FieldSource `json:"fields"`
	Results []ProviderResult       `json:"providers"`
}
