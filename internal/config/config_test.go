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
