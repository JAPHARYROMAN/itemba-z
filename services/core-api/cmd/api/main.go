package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/advancedfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/banking"
	"github.com/itemba-z/itemba-z/services/core-api/internal/commercial"
	"github.com/itemba-z/itemba-z/services/core-api/internal/configuration"
	"github.com/itemba-z/itemba-z/services/core-api/internal/financialops"
	"github.com/itemba-z/itemba-z/services/core-api/internal/groupfinance"
	"github.com/itemba-z/itemba-z/services/core-api/internal/httpapi"
	"github.com/itemba-z/itemba-z/services/core-api/internal/integrations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/inventorycontrol"
	"github.com/itemba-z/itemba-z/services/core-api/internal/mobile"
	"github.com/itemba-z/itemba-z/services/core-api/internal/operations"
	"github.com/itemba-z/itemba-z/services/core-api/internal/people"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/clock"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/memory"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/store/postgres"
	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/telemetry"
	"github.com/itemba-z/itemba-z/services/core-api/internal/readmodel"
	"github.com/itemba-z/itemba-z/services/core-api/internal/receivables"
	"github.com/itemba-z/itemba-z/services/core-api/internal/reporting"
	"github.com/itemba-z/itemba-z/services/core-api/internal/sales"
	"github.com/itemba-z/itemba-z/services/core-api/internal/treasury"
	"github.com/itemba-z/itemba-z/services/core-api/internal/workforce"
)

func main() {
	logger := telemetry.NewJSONLogger(os.Stdout)
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
		maxAuthAgeSeconds, parseErr := strconv.Atoi(os.Getenv("OIDC_MAX_AUTH_AGE_SECONDS"))
		if parseErr != nil || maxAuthAgeSeconds < 60 || maxAuthAgeSeconds > 86400 {
			logger.Error("OIDC_MAX_AUTH_AGE_SECONDS must be an integer between 60 and 86400")
			os.Exit(1)
		}
		requiredAMR := strings.FieldsFunc(os.Getenv("OIDC_REQUIRED_AMR"), func(r rune) bool { return r == ',' || r == ' ' })
		policy := httpapi.OIDCAssurancePolicy{RequiredACR: os.Getenv("OIDC_REQUIRED_ACR"), RequiredAMR: requiredAMR, MaxAuthAge: time.Duration(maxAuthAgeSeconds) * time.Second}
		oidcAuthenticator, err := httpapi.NewOIDCAuthenticator(startupContext, issuer, audience, policy)
		if err != nil {
			logger.Error("initialize OIDC authentication", "error", err)
			os.Exit(1)
		}
		authenticator = oidcAuthenticator
	} else {
		if !development {
			logger.Error("OIDC issuer, audience, MFA assurance policy and authentication age are required outside explicit development", "environment", environment)
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
	receivablesRepository, ok := repository.(receivables.Repository)
	if !ok {
		logger.Error("repository does not implement customer receivables")
		os.Exit(1)
	}
	receivablesService, err := receivables.NewService(receivablesRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize receivables service", "error", err)
		os.Exit(1)
	}
	operationsRepository, ok := repository.(operations.Repository)
	if !ok {
		logger.Error("repository does not implement commercial operations")
		os.Exit(1)
	}
	operationsService, err := operations.NewService(operationsRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize operations service", "error", err)
		os.Exit(1)
	}
	bankingRepository, ok := repository.(banking.Repository)
	if !ok {
		logger.Error("repository does not implement cash and bank reconciliation")
		os.Exit(1)
	}
	bankingService, err := banking.NewService(bankingRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize banking service", "error", err)
		os.Exit(1)
	}
	financialRepository, ok := repository.(financialops.Repository)
	if !ok {
		logger.Error("repository does not implement governed financial operations")
		os.Exit(1)
	}
	financialService, err := financialops.NewService(financialRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize financial operations service", "error", err)
		os.Exit(1)
	}
	reportingRepository, ok := repository.(reporting.Repository)
	if !ok {
		logger.Error("repository does not implement governed financial reporting")
		os.Exit(1)
	}
	reportingService, err := reporting.NewService(reportingRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize financial reporting service", "error", err)
		os.Exit(1)
	}
	advancedRepository, ok := repository.(advancedfinance.Repository)
	if !ok {
		logger.Error("repository does not implement advanced finance")
		os.Exit(1)
	}
	advancedService, err := advancedfinance.NewService(advancedRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize advanced finance service", "error", err)
		os.Exit(1)
	}
	treasuryRepository, ok := repository.(treasury.Repository)
	if !ok {
		logger.Error("repository does not implement treasury")
		os.Exit(1)
	}
	treasuryService, err := treasury.NewService(treasuryRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize treasury service", "error", err)
		os.Exit(1)
	}
	groupRepository, ok := repository.(groupfinance.Repository)
	if !ok {
		logger.Error("repository does not implement group finance")
		os.Exit(1)
	}
	groupService, err := groupfinance.NewService(groupRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize group finance service", "error", err)
		os.Exit(1)
	}
	peopleRepository, ok := repository.(people.Repository)
	if !ok {
		logger.Error("repository does not implement people and payroll")
		os.Exit(1)
	}
	peopleService, err := people.NewService(peopleRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize people service", "error", err)
		os.Exit(1)
	}
	configurationRepository, ok := repository.(configuration.Repository)
	if !ok {
		logger.Error("repository does not implement governed configuration")
		os.Exit(1)
	}
	configurationService, err := configuration.NewService(configurationRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize configuration service", "error", err)
		os.Exit(1)
	}
	commercialRepository, ok := repository.(commercial.Repository)
	if !ok {
		logger.Error("repository does not implement commercial sourcing")
		os.Exit(1)
	}
	commercialService, err := commercial.NewService(commercialRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize commercial service", "error", err)
		os.Exit(1)
	}
	inventoryRepository, ok := repository.(inventorycontrol.Repository)
	if !ok {
		logger.Error("repository does not implement inventory control")
		os.Exit(1)
	}
	inventoryService, err := inventorycontrol.NewService(inventoryRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize inventory control service", "error", err)
		os.Exit(1)
	}
	workforceRepository, ok := repository.(workforce.Repository)
	if !ok {
		logger.Error("repository does not implement workforce control")
		os.Exit(1)
	}
	workforceService, err := workforce.NewService(workforceRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize workforce service", "error", err)
		os.Exit(1)
	}
	integrationRepository, ok := repository.(integrations.OperationsRepository)
	if !ok {
		logger.Error("repository does not implement integration operations")
		os.Exit(1)
	}
	integrationService, err := integrations.NewService(integrationRepository, identity.UUIDGenerator{}, clock.System{})
	if err != nil {
		logger.Error("initialize integration operations service", "error", err)
		os.Exit(1)
	}
	handler, err := httpapi.NewLiveWithIntegrationOperations(salesService, readService, mobileService, receivablesService, operationsService, bankingService, financialService, reportingService, advancedService, treasuryService, groupService, peopleService, configurationService, commercialService, inventoryService, workforceService, integrationService, logger, authenticator)
	if err != nil {
		logger.Error("initialize HTTP API", "error", err)
		os.Exit(1)
	}
	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metricsRegistry := telemetry.NewRegistry()
	handler.SetHTTPObserver(metricsRegistry)
	metricsAddress := strings.TrimSpace(os.Getenv("ITEMBA_METRICS_ADDRESS"))
	if metricsAddress == "" && !development {
		logger.Error("ITEMBA_METRICS_ADDRESS is required outside explicit development")
		os.Exit(1)
	}
	var metricsServer *http.Server
	metricsFailure := make(chan error, 1)
	if metricsAddress != "" {
		listener, listenErr := net.Listen("tcp", metricsAddress)
		if listenErr != nil {
			logger.Error("bind internal metrics listener", "error", listenErr)
			os.Exit(1)
		}
		metricsServer = &http.Server{Addr: metricsAddress, Handler: metricsRegistry.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
		go func() {
			if serveErr := metricsServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				logger.Error("internal metrics server stopped", "error", serveErr)
				metricsFailure <- serveErr
				stop()
			}
		}()
		logger.Info("ITEMBA-Z internal metrics listening", "address", metricsAddress)
	}
	address := os.Getenv("ITEMBA_HTTP_ADDRESS")
	if address == "" {
		address = ":8080"
	}
	server := &http.Server{
		Addr: address, Handler: handler.Routes(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	if store, ok := repository.(*postgres.Store); ok {
		updatePoolMetrics := func() {
			statistics := store.Pool().Stat()
			metricsRegistry.UpdateDatabasePool(statistics.AcquiredConns(), statistics.IdleConns(), statistics.TotalConns(), statistics.MaxConns())
			queryContext, cancel := context.WithTimeout(shutdownSignal, 5*time.Second)
			defer cancel()
			operationalMetrics, queryErr := store.PlatformOperationalMetrics(queryContext)
			if queryErr != nil {
				logger.Warn("refresh platform operational metrics", "error", queryErr)
				return
			}
			metricsRegistry.ReplaceOperationalMetrics(operationalMetrics)
		}
		updatePoolMetrics()
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-shutdownSignal.Done():
					return
				case <-ticker.C:
					updatePoolMetrics()
				}
			}
		}()
	}
	go func() {
		<-shutdownSignal.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown", "error", err)
		}
		if metricsServer != nil {
			if err := metricsServer.Shutdown(ctx); err != nil {
				logger.Error("metrics graceful shutdown", "error", err)
			}
		}
	}()
	logger.Info("ITEMBA-Z core API listening", "address", address, "environment", environment)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
	select {
	case err := <-metricsFailure:
		logger.Error("core API stopped because required metrics failed", "error", err)
		os.Exit(1)
	default:
	}
}

func allowsUnsafeFallback(environment string) bool { return environment == "development" }
