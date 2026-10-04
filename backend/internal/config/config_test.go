package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"JWT_SECRET":           strings.Repeat("k", 32),
		"JWT_TTL":              "5m",
		"CORS_ALLOWED_ORIGINS": "http://a, http://b",
		"ABC_BASE_URL":         "http://abc", "ABC_USERNAME": "u", "ABC_PASSWORD": "p",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBDriver != "sqlite" || cfg.HTTPAddr != ":8080" || cfg.JWTTTL != 5*time.Minute {
		t.Fatalf("unexpected %+v", cfg)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || !cfg.ABC.Enabled() || cfg.XYC.Enabled() {
		t.Fatalf("unexpected %+v", cfg)
	}
}

func TestLoadValidation(t *testing.T) {
	_, err := load(env(map[string]string{
		"JWT_SECRET":          "short",
		"DB_DRIVER":           "mysql",
		"JWT_TTL":             "soon",
		"XYC_BASE_URL":        "http://xyc",
		"LOG_LEVEL":           "loud",
		"TRUSTED_PROXY_CIDRS": "10.0.0.0/8, not-a-cidr",
	}))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"JWT_SECRET", "DB_DRIVER", "JWT_TTL", "XYC_USERNAME", "LOG_LEVEL", "not-a-cidr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %s: %v", want, err)
		}
	}
}

func TestLoadCredentialsDatabase(t *testing.T) {
	secret := strings.Repeat("k", 32)
	cfg, err := load(env(map[string]string{"JWT_SECRET": secret}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL == cfg.CredentialsDBURL {
		t.Fatalf("sqlite defaults share a file: %q", cfg.DatabaseURL)
	}

	cases := map[string]map[string]string{
		"postgres needs both URLs": {"JWT_SECRET": secret, "DB_DRIVER": "postgres", "DATABASE_URL": "postgres://p/profiles"},
		"same database rejected": {
			"JWT_SECRET": secret, "DATABASE_URL": "postgres://p/db", "CREDENTIALS_DATABASE_URL": "postgres://p/db",
		},
	}
	for name, vars := range cases {
		if _, err := load(env(vars)); err == nil || !strings.Contains(err.Error(), "CREDENTIALS_DATABASE_URL") {
			t.Errorf("%s: want CREDENTIALS_DATABASE_URL error, got %v", name, err)
		}
	}
}
