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

	"github.com/itemba-z/itemba-z/services/core-api/internal/httpapi"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	environment := os.Getenv("ITEMBA_ENV")
	development := allowsUnsafeFallback(environment)

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStartup()
	var repository sales.Repository
	closeRepository := func() {}
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := postgres.Open(startupContext, databaseURL)
		if err != nil {
			logger.Error("initialize PostgreSQL", "error", err)
			os.Exit(1)
		}
		repository = store
		closeRepository = store.Close
		logger.Info("PostgreSQL repository initialized")
	} else {
		if !development {
			logger.Error("DATABASE_URL is required outside explicit development", "environment", environment)
			os.Exit(1)
		}
		repository = memory.New()
		logger.Warn("using non-durable in-memory repository", "environment", environment)
	}
	defer closeRepository()

	var authenticator httpapi.Authenticator
	issuer, audience := os.Getenv("OIDC_ISSUER_URL"), os.Getenv("OIDC_AUDIENCE")
	if issuer != "" || audience != "" {
		oidcAuthenticator, err := httpapi.NewOIDCAuthenticator(startupContext, issuer, audience)
		if err != nil {
			logger.Error("initialize OIDC authentication", "error", err)
			os.Exit(1)
		}
		authenticator = oidcAuthenticator
	} else {
		if !development {
			logger.Error("OIDC_ISSUER_URL and OIDC_AUDIENCE are required outside explicit development", "environment", environment)
			os.Exit(1)
		}
		authenticator = httpapi.DevelopmentHeaderAuthenticator{}
		logger.Warn("using development-only header authentication")
	}
	salesService, err := sales.NewService(repository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize sales service", "error", err)
		os.Exit(1)
	}
	readRepository, ok := repository.(readmodel.Repository)
	if !ok {
		logger.Error("repository does not implement live read models")
		os.Exit(1)
	}
	readService, err := readmodel.NewService(readRepository)
	if err != nil {
		logger.Error("initialize read service", "error", err)
		os.Exit(1)
	}
	mobileRepository, ok := repository.(mobile.Repository)
	if !ok {
		logger.Error("repository does not implement mobile enrollment")
		os.Exit(1)
	}
	mobileService, err := mobile.NewService(mobileRepository, salesService, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize mobile service", "error", err)
		os.Exit(1)
	}
	handler, err := httpapi.NewLive(salesService, readService, mobileService, logger, authenticator)
	if err != nil {
		logger.Error("initialize HTTP API", "error", err)
		os.Exit(1)
	}
	address := os.Getenv("ITEMBA_HTTP_ADDRESS")
	if address == "" {
		address = ":8080"
	}
	server := &http.Server{
		Addr: address, Handler: handler.Routes(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownSignal.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown", "error", err)
		}
	}()
	logger.Info("ITEMBA-Z core API listening", "address", address, "environment", environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}

func allowsUnsafeFallback(environment string) bool { return environment == "development" }
