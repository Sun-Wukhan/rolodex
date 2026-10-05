// Package config loads runtime configuration from environment variables and
// validates it at startup so misconfiguration fails fast.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"time"
)

// ProviderConfig holds connection settings for one identity provider.
type ProviderConfig struct {
	BaseURL  string
	Username string
	Password string
}

// Enabled reports whether the provider has been configured.
func (p ProviderConfig) Enabled() bool { return p.BaseURL != "" }

// Config is the full application configuration.
type Config struct {
	HTTPAddr            string
	DBDriver            string
	DatabaseURL         string
	DatabaseReadURL     string
	CredentialsDBURL    string
	JWTSecret           []byte
	JWTIssuer           string
	JWTTTL              time.Duration
	CORSAllowedOrigins  []string
	TrustedProxyCIDRs   []string
	LoginRatePerMinute  int
	ProviderTimeout     time.Duration
	ProviderHTTPTimeout time.Duration
	ABC                 ProviderConfig
	XYC                 ProviderConfig
	LogLevel            slog.Level
	Firebase            FirebaseConfig
}

// FirebaseConfig enables Google sign-in through Firebase Authentication.
type FirebaseConfig struct {
	ProjectID      string
	AllowedEmails  []string
	AllowedDomains []string
}

// Enabled reports whether Firebase sign-in has been configured.
func (f FirebaseConfig) Enabled() bool { return f.ProjectID != "" }

var firebaseProjectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

// Load reads configuration from the environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	var errs []error
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	dur := func(key string, def time.Duration) time.Duration {
		raw := get(key, "")
		if raw == "" {
			return def
		}
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s: invalid duration %q", key, raw))
			return def
		}
		return d
	}

	cfg := Config{
		HTTPAddr:            get("HTTP_ADDR", ":8080"),
		DBDriver:            get("DB_DRIVER", "sqlite"),
		DatabaseURL:         get("DATABASE_URL", "rolodex.db"),
		DatabaseReadURL:     get("DATABASE_READ_URL", ""),
		CredentialsDBURL:    get("CREDENTIALS_DATABASE_URL", "rolodex-credentials.db"),
		JWTSecret:           []byte(get("JWT_SECRET", "")),
		JWTIssuer:           get("JWT_ISSUER", "rolodex"),
		JWTTTL:              dur("JWT_TTL", 15*time.Minute),
		CORSAllowedOrigins:  splitCSV(get("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		TrustedProxyCIDRs:   splitCSV(get("TRUSTED_PROXY_CIDRS", "")),
		ProviderTimeout:     dur("PROVIDER_TIMEOUT", 8*time.Second),
		ProviderHTTPTimeout: dur("PROVIDER_HTTP_TIMEOUT", 3*time.Second),
		ABC:                 ProviderConfig{BaseURL: get("ABC_BASE_URL", ""), Username: get("ABC_USERNAME", ""), Password: get("ABC_PASSWORD", "")},
		XYC:                 ProviderConfig{BaseURL: get("XYC_BASE_URL", ""), Username: get("XYC_USERNAME", ""), Password: get("XYC_PASSWORD", "")},
		LoginRatePerMinute:  10,
		Firebase: FirebaseConfig{
			ProjectID:      get("FIREBASE_PROJECT_ID", ""),
			AllowedEmails:  splitCSV(get("FIREBASE_ALLOWED_EMAILS", "")),
			AllowedDomains: splitCSV(get("FIREBASE_ALLOWED_DOMAINS", "")),
		},
	}

	if _, err := fmt.Sscanf(get("LOGIN_RATE_PER_MINUTE", "10"), "%d", &cfg.LoginRatePerMinute); err != nil || cfg.LoginRatePerMinute <= 0 {
		errs = append(errs, errors.New("LOGIN_RATE_PER_MINUTE: must be a positive integer"))
	}
	for _, cidr := range cfg.TrustedProxyCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			errs = append(errs, fmt.Errorf("TRUSTED_PROXY_CIDRS: invalid CIDR %q", cidr))
		}
	}
	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET: required, at least 32 bytes"))
	}
	if cfg.DBDriver != "postgres" && cfg.DBDriver != "sqlite" {
		errs = append(errs, fmt.Errorf("DB_DRIVER: must be postgres or sqlite, got %q", cfg.DBDriver))
	}
	if cfg.DBDriver == "postgres" && (get("DATABASE_URL", "") == "" || get("CREDENTIALS_DATABASE_URL", "") == "") {
		errs = append(errs, errors.New("DATABASE_URL and CREDENTIALS_DATABASE_URL: both required with DB_DRIVER=postgres"))
	}
	if cfg.DatabaseReadURL != "" && cfg.DBDriver != "postgres" {
		errs = append(errs, errors.New("DATABASE_READ_URL: read replicas require DB_DRIVER=postgres"))
	}
	if cfg.DatabaseReadURL != "" && cfg.DatabaseReadURL == cfg.DatabaseURL {
		errs = append(errs, errors.New("DATABASE_READ_URL: must point at a replica, not the primary DATABASE_URL"))
	}
	if cfg.DatabaseURL == cfg.CredentialsDBURL {
		errs = append(errs, errors.New("CREDENTIALS_DATABASE_URL: must point at a different database than DATABASE_URL"))
	}
	for name, p := range map[string]ProviderConfig{"ABC": cfg.ABC, "XYC": cfg.XYC} {
		if p.Enabled() && (p.Username == "" || p.Password == "") {
			errs = append(errs, fmt.Errorf("%s_USERNAME and %s_PASSWORD are required when %s_BASE_URL is set", name, name, name))
		}
	}
	allowlisted := len(cfg.Firebase.AllowedEmails)+len(cfg.Firebase.AllowedDomains) > 0
	switch {
	case cfg.Firebase.Enabled() && !firebaseProjectPattern.MatchString(cfg.Firebase.ProjectID):
		errs = append(errs, fmt.Errorf("FIREBASE_PROJECT_ID: invalid project ID %q", cfg.Firebase.ProjectID))
	case cfg.Firebase.Enabled() && !allowlisted:
		errs = append(errs, errors.New("FIREBASE_ALLOWED_EMAILS or FIREBASE_ALLOWED_DOMAINS: required with FIREBASE_PROJECT_ID, otherwise nobody could sign in"))
	case !cfg.Firebase.Enabled() && allowlisted:
		errs = append(errs, errors.New("FIREBASE_PROJECT_ID: required when a Firebase allowlist is set"))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	return cfg, errors.Join(errs...)
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
