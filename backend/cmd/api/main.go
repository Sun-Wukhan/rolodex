// Command api runs the Rolodex REST API. This is the composition root: it is
// the only place where concrete implementations are chosen and wired together.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sun-Wukhan/rolodex/internal/config"
	"github.com/Sun-Wukhan/rolodex/internal/httpapi"
	"github.com/Sun-Wukhan/rolodex/internal/provider"
	"github.com/Sun-Wukhan/rolodex/internal/provider/abc"
	"github.com/Sun-Wukhan/rolodex/internal/provider/xyc"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/internal/repository/factory"
	"github.com/Sun-Wukhan/rolodex/internal/security"
	"github.com/Sun-Wukhan/rolodex/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	repo, err := factory.Open(startCtx, cfg.DBDriver, cfg.DatabaseURL, cfg.CredentialsDBURL,
		factory.WithProfilesReplica(cfg.DatabaseReadURL, log))
	if err != nil {
		return err
	}
	defer func() { _ = repo.Close() }()
	log.Info("datastores ready", "driver", cfg.DBDriver, "databases", []string{"profiles", "credentials"},
		"profiles_read_replica", cfg.DatabaseReadURL != "")

	hasher := security.NewArgon2Hasher(security.DefaultArgon2Params())
	tokens, err := security.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	if err != nil {
		return err
	}

	auth, err := service.NewAuthService(repo, hasher, tokens, log)
	if err != nil {
		return err
	}
	profiles := service.NewProfileService(repo, hasher)
	identity := service.NewIdentityService(repo, buildProviders(cfg, log), cfg.ProviderTimeout, log)
	firebase, err := buildFirebase(cfg, repo, tokens, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Auth: auth, Firebase: firebase, Profiles: profiles, Identity: identity, Ready: repo, Tokens: tokens, Log: log,
			CORSAllowedOrigins: cfg.CORSAllowedOrigins, LoginRatePerMinute: cfg.LoginRatePerMinute,
			TrustedProxyCIDRs: cfg.TrustedProxyCIDRs,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "providers", identity.ProviderNames())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	return srv.Shutdown(shutdownCtx)
}

// buildFirebase returns the Google sign-in authenticator, or nil (route
// disabled) when FIREBASE_PROJECT_ID is unset.
func buildFirebase(cfg config.Config, repo repository.UserRepository, tokens service.TokenIssuer, log *slog.Logger) (httpapi.FirebaseAuthenticator, error) {
	if !cfg.Firebase.Enabled() {
		return nil, nil
	}
	verifier, err := security.NewFirebaseVerifier(cfg.Firebase.ProjectID, security.FirebaseKeysURL, nil)
	if err != nil {
		return nil, err
	}
	allow := service.NewEmailAllowlist(cfg.Firebase.AllowedEmails, cfg.Firebase.AllowedDomains)
	log.Info("firebase sign-in enabled", "project_id", cfg.Firebase.ProjectID,
		"allowed_emails", len(cfg.Firebase.AllowedEmails), "allowed_domains", cfg.Firebase.AllowedDomains)
	if allow.AllowsAny() {
		log.Warn("firebase sign-in admits any verified Google account", "setting", "FIREBASE_ALLOWED_DOMAINS="+service.AnyDomain)
	}
	return service.NewFirebaseAuthService(repo, verifier, allow, tokens, log), nil
}

func buildProviders(cfg config.Config, log *slog.Logger) []provider.IdentityProvider {
	clientCfg := func(p config.ProviderConfig) provider.ClientConfig {
		return provider.ClientConfig{
			BaseURL:     p.BaseURL,
			Credentials: provider.Credentials{Username: p.Username, Password: p.Password},
			Timeout:     cfg.ProviderHTTPTimeout,
			Retry:       provider.DefaultRetryPolicy(),
		}
	}
	var out []provider.IdentityProvider
	if cfg.ABC.Enabled() {
		out = append(out, abc.New(clientCfg(cfg.ABC)))
	}
	if cfg.XYC.Enabled() {
		out = append(out, xyc.New(clientCfg(cfg.XYC)))
	}
	if len(out) == 0 {
		log.Warn("no identity providers configured; enrichment disabled")
	}
	return out
}
