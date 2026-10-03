// Package config loads runtime configuration from environment variables and
// validates it at startup so misconfiguration fails fast.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
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
}

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
	for name, p := range map[string]ProviderConfig{"ABC": cfg.ABC, "XYC": cfg.XYC} {
		if p.Enabled() && (p.Username == "" || p.Password == "") {
			errs = append(errs, fmt.Errorf("%s_USERNAME and %s_PASSWORD are required when %s_BASE_URL is set", name, name, name))
		}
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
