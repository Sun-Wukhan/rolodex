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

func TestLoadFirebase(t *testing.T) {
	secret := strings.Repeat("k", 32)
	cfg, err := load(env(map[string]string{"JWT_SECRET": secret}))
	if err != nil || cfg.Firebase.Enabled() {
		t.Fatalf("firebase must be off by default: %+v %v", cfg.Firebase, err)
	}

	cfg, err = load(env(map[string]string{
		"JWT_SECRET": secret, "FIREBASE_PROJECT_ID": "grand-exchange-b10eb",
		"FIREBASE_ALLOWED_EMAILS": "ada@example.com, grace@example.com", "FIREBASE_ALLOWED_DOMAINS": "navy.mil",
	}))
	if err != nil || !cfg.Firebase.Enabled() || len(cfg.Firebase.AllowedEmails) != 2 || len(cfg.Firebase.AllowedDomains) != 1 {
		t.Fatalf("got %+v, %v", cfg.Firebase, err)
	}

	for name, vars := range map[string]map[string]string{
		"project without allowlist": {"FIREBASE_PROJECT_ID": "grand-exchange-b10eb"},
		"allowlist without project": {"FIREBASE_ALLOWED_DOMAINS": "navy.mil"},
		"invalid project ID":        {"FIREBASE_PROJECT_ID": "Not A Project", "FIREBASE_ALLOWED_DOMAINS": "navy.mil"},
	} {
		vars["JWT_SECRET"] = secret
		if _, err := load(env(vars)); err == nil || !strings.Contains(err.Error(), "FIREBASE_") {
			t.Errorf("%s: want FIREBASE_ error, got %v", name, err)
		}
	}
}

func TestLoadReadReplica(t *testing.T) {
	base := map[string]string{
		"JWT_SECRET": strings.Repeat("k", 32), "DB_DRIVER": "postgres",
		"DATABASE_URL": "postgres://primary/p", "CREDENTIALS_DATABASE_URL": "postgres://creds/c",
	}
	with := func(extra map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	cfg, err := load(env(with(map[string]string{"DATABASE_READ_URL": "postgres://replica/p"})))
	if err != nil || cfg.DatabaseReadURL != "postgres://replica/p" {
		t.Fatalf("got %q, %v", cfg.DatabaseReadURL, err)
	}
	for name, extra := range map[string]map[string]string{
		"replica is primary": {"DATABASE_READ_URL": "postgres://primary/p"},
		"sqlite replica":     {"DATABASE_READ_URL": "r.db", "DB_DRIVER": "sqlite"},
	} {
		if _, err := load(env(with(extra))); err == nil || !strings.Contains(err.Error(), "DATABASE_READ_URL") {
			t.Errorf("%s: want DATABASE_READ_URL error, got %v", name, err)
		}
	}
}
