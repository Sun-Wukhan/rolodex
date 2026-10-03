package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/internal/repository"
	"github.com/navid/rolodex/internal/security"
)

const (
	minPasswordLen = 12
	maxPasswordLen = 128
	maxNameLen     = 200
	maxFieldLen    = 200
)

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9._@-]{3,64}$`)
	countryPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
)

// CreateUserInput is the data needed to register a user with a password.
type CreateUserInput struct {
	Name     string         `json:"name"`
	Phone    string         `json:"phone"`
	Address  domain.Address `json:"address"`
	Username string         `json:"username"`
	Password string         `json:"password"`
}

// UserDetails is a profile together with its credential metadata (never secrets).
type UserDetails struct {
	Profile     domain.Profile      `json:"profile"`
	Credentials []domain.Credential `json:"credentials"`
}

// ValidationError lists per-field validation problems.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }

// Unwrap lets callers match ValidationError with errors.Is(err, domain.ErrInvalidInput).
func (e *ValidationError) Unwrap() error { return domain.ErrInvalidInput }

// ProfileService manages user profiles and search.
type ProfileService struct {
	repo   repository.UserRepository
	hasher security.PasswordHasher
}

// NewProfileService wires a ProfileService.
func NewProfileService(repo repository.UserRepository, hasher security.PasswordHasher) *ProfileService {
	return &ProfileService{repo: repo, hasher: hasher}
}

// CreateUser validates and normalises input, hashes the password and stores
// the user, profile and password credential atomically.
func (s *ProfileService) CreateUser(ctx context.Context, in CreateUserInput) (domain.User, error) {
	profile, username, err := validateCreate(in)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, fmt.Errorf("profile: hash: %w", err)
	}
	return s.repo.CreateUser(ctx, profile, domain.Credential{
		Method: domain.MethodPassword, Username: username, SecretHash: hash,
	})
}

// GetUser returns a profile and its credential metadata.
func (s *ProfileService) GetUser(ctx context.Context, id uuid.UUID) (*UserDetails, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	creds, err := s.repo.ListCredentials(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UserDetails{Profile: *p, Credentials: creds}, nil
}

// Search finds profiles by name, phone and/or username. At least one filter is
// required to prevent unbounded enumeration of PII.
func (s *ProfileService) Search(ctx context.Context, q domain.SearchQuery) ([]domain.Profile, error) {
	q.Name = strings.TrimSpace(q.Name)
	q.Username = strings.ToLower(strings.TrimSpace(q.Username))
	q.Phone = strings.TrimSpace(q.Phone)

	if q.IsEmpty() {
		return nil, &ValidationError{Fields: map[string]string{"query": "provide at least one of name, phone or username"}}
	}
	if utf8.RuneCountInString(q.Name) > maxNameLen || len(q.Username) > 64 {
		return nil, &ValidationError{Fields: map[string]string{"query": "search term too long"}}
	}
	if q.Phone != "" {
		phone, err := domain.NormalizePhone(q.Phone)
		if err != nil {
			return nil, &ValidationError{Fields: map[string]string{"phone": "invalid phone number"}}
		}
		q.Phone = phone
	}
	return s.repo.SearchProfiles(ctx, q.Normalize())
}

func validateCreate(in CreateUserInput) (domain.Profile, string, error) {
	errs := map[string]string{}

	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		errs["name"] = "required, at most 200 characters"
	}
	phone, err := domain.NormalizePhone(in.Phone)
	if err != nil {
		errs["phone"] = "invalid phone number"
	}
	username := strings.ToLower(strings.TrimSpace(in.Username))
	if !usernamePattern.MatchString(username) {
		errs["username"] = "3-64 characters: letters, digits, . _ @ -"
	}
	if n := utf8.RuneCountInString(in.Password); n < minPasswordLen || n > maxPasswordLen {
		errs["password"] = fmt.Sprintf("must be %d-%d characters", minPasswordLen, maxPasswordLen)
	}

	addr := domain.Address{
		StreetAddress: strings.TrimSpace(in.Address.StreetAddress),
		Locality:      strings.TrimSpace(in.Address.Locality),
		Region:        strings.TrimSpace(in.Address.Region),
		PostalCode:    strings.TrimSpace(in.Address.PostalCode),
		Country:       strings.ToUpper(strings.TrimSpace(in.Address.Country)),
	}
	for field, v := range map[string]string{
		"address.street_address": addr.StreetAddress, "address.locality": addr.Locality,
		"address.region": addr.Region, "address.postal_code": addr.PostalCode,
	} {
		if utf8.RuneCountInString(v) > maxFieldLen {
			errs[field] = "at most 200 characters"
		}
	}
	if addr.Country != "" && !countryPattern.MatchString(addr.Country) {
		errs["address.country"] = "ISO 3166-1 alpha-2 code"
	}

	if len(errs) > 0 {
		return domain.Profile{}, "", &ValidationError{Fields: errs}
	}
	return domain.Profile{Name: name, Phone: phone, Address: addr}, username, nil
}
