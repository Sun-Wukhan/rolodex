// Command mockvendors runs fake ABC and XYC identity vendors on separate ports
// so the API exercises real HTTP integrations locally.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Sun-Wukhan/rolodex/internal/mockvendor"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	vendors := []struct {
		addr string
		cfg  mockvendor.Config
		recs []mockvendor.Record
	}{
		{
			addr: env("ABC_ADDR", ":9001"),
			cfg: mockvendor.Config{
				Format: mockvendor.FormatABC, Username: mustEnv(log, "MOCK_ABC_USERNAME"), Password: mustEnv(log, "MOCK_ABC_PASSWORD"),
				FailureRate: floatEnv(log, "ABC_FAILURE_RATE", 0), Latency: durEnv(log, "ABC_LATENCY", 50*time.Millisecond),
			},
			recs: mockvendor.ABCRecords(),
		},
		{
			addr: env("XYC_ADDR", ":9002"),
			cfg: mockvendor.Config{
				Format: mockvendor.FormatXYC, Username: mustEnv(log, "MOCK_XYC_USERNAME"), Password: mustEnv(log, "MOCK_XYC_PASSWORD"),
				FailureRate: floatEnv(log, "XYC_FAILURE_RATE", 0.3), Latency: durEnv(log, "XYC_LATENCY", 150*time.Millisecond),
			},
			recs: mockvendor.XYCRecords(),
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	servers := make([]*http.Server, 0, len(vendors))
	for _, v := range vendors {
		srv := &http.Server{
			Addr:              v.addr,
			Handler:           mockvendor.New(v.cfg, v.recs, log).Handler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		servers = append(servers, srv)
		go func(name string) {
			log.Info("mock vendor listening", "vendor", name, "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("mock vendor failed", "vendor", name, "error", err)
				stop()
			}
		}(string(v.cfg.Format))
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range servers {
		_ = s.Shutdown(shutdownCtx)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(log *slog.Logger, key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Error("missing required environment variable", "key", key)
		os.Exit(1)
	}
	return v
}

func floatEnv(log *slog.Logger, key string, def float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f < 0 || f > 1 {
		log.Error("invalid float in range [0,1]", "key", key)
		os.Exit(1)
	}
	return f
}

func durEnv(log *slog.Logger, key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		log.Error("invalid duration", "key", key)
		os.Exit(1)
	}
	return d
}
