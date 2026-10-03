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

	"github.com/navid/rolodex/internal/config"
	"github.com/navid/rolodex/internal/httpapi"
	"github.com/navid/rolodex/internal/provider"
	"github.com/navid/rolodex/internal/provider/abc"
	"github.com/navid/rolodex/internal/provider/xyc"
	"github.com/navid/rolodex/internal/repository/factory"
	"github.com/navid/rolodex/internal/security"
	"github.com/navid/rolodex/internal/service"
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
	repo, err := factory.Open(startCtx, cfg.DBDriver, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = repo.Close() }()
	log.Info("datastore ready", "driver", cfg.DBDriver)

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

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Auth: auth, Profiles: profiles, Identity: identity, Ready: repo, Tokens: tokens, Log: log,
			CORSAllowedOrigins: cfg.CORSAllowedOrigins, LoginRatePerMinute: cfg.LoginRatePerMinute,
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
