package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"maragu.dev/goqite"
	"maragu.dev/goqite/jobs"

	"github.com/grantsy/grantsy/internal/auth"
	"github.com/grantsy/grantsy/internal/entitlements"
	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/grantsy/grantsy/internal/infra/config"
	"github.com/grantsy/grantsy/internal/infra/db"
	"github.com/grantsy/grantsy/internal/infra/logger"
	"github.com/grantsy/grantsy/internal/infra/metrics"
	"github.com/grantsy/grantsy/internal/infra/server"
	"github.com/grantsy/grantsy/internal/infra/tracing"
	_ "github.com/grantsy/grantsy/internal/infra/validation"
	"github.com/grantsy/grantsy/internal/openapi"
	"github.com/grantsy/grantsy/internal/subscriptions"
	"github.com/grantsy/grantsy/internal/users"
	"github.com/grantsy/grantsy/internal/webhooks"
	"github.com/grantsy/grantsy/pkg/gracefulshutdown"
)

const healthcheckProbePath = "/healthz"

func main() {
	//
	// Infra
	//

	gracefulshutdown.SubscribeForShutdown()

	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Log.Level, cfg.Log.Format, cfg.Env)
	slog.Debug("starting grantsy", "config", *cfg)

	if err := db.Migrate(cfg.Database.Driver, cfg.Database.DSN); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	database, err := db.New(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	//
	// Services
	//

	var flavor goqite.SQLFlavor
	switch cfg.Database.Driver {
	case "postgres":
		flavor = goqite.SQLFlavorPostgreSQL
	case "sqlite":
		flavor = goqite.SQLFlavorSQLite
	default:
		slog.Error("unsupported database driver", "driver", cfg.Database.Driver)
		os.Exit(1)
	}

	webhookQueue := goqite.New(goqite.NewOpts{
		DB:         database.DB,
		Name:       "webhooks",
		SQLFlavor:  flavor,
		MaxReceive: 5,
		Timeout:    time.Second * 15,
	})

	// Create services (order matters for DI chain)
	subsRepo := subscriptions.NewRepo(database)

	webhookService := webhooks.NewService(webhookQueue, cfg.Webhooks.Endpoints)

	lsProvider := subscriptions.NewLemonSqueezyProvider(
		cfg.Providers.LemonSqueezy.APIKey,
		cfg.Providers.LemonSqueezy.Webhook.Secret,
		cfg.Providers.LemonSqueezy.Products,
	)

	var syncPeriod time.Duration
	if cfg.SyncPeriod != "" {
		syncPeriod, err = time.ParseDuration(cfg.SyncPeriod)
		if err != nil {
			slog.Error("failed to parse sync_period", "error", err)
			os.Exit(1)
		}
	}
	go lsProvider.Start(gracefulshutdown.GetServerBaseContext(), syncPeriod)

	entService, err := entitlements.NewService(
		&cfg.Entitlements,
		cfg.Providers.LemonSqueezy.Products,
		subsRepo,
		webhookService,
	)
	if err != nil {
		slog.Error("failed to create entitlements service", "error", err)
		os.Exit(1)
	}

	// Start webhook worker
	webhookWorker := webhooks.NewWorker(cfg.Webhooks.Endpoints)
	runner := jobs.NewRunner(jobs.NewRunnerOpts{
		Limit:        10,
		PollInterval: time.Second,
		Queue:        webhookQueue,
		Log:          slog.Default(),
	})
	runner.Register("webhooks", webhookWorker.Handle)
	go runner.Start(gracefulshutdown.GetServerBaseContext())

	//
	// Routes
	//

	reflector := openapi.NewReflector()

	routes := []httptools.Route{
		entitlements.NewRouteCheck(entService),
		entitlements.NewRouteFeatures(entService),
		entitlements.NewRouteFeature(entService),
		entitlements.NewRoutePlans(entService, lsProvider),
		entitlements.NewRoutePlan(entService, lsProvider),
		users.NewRouteUser(entService, subsRepo),
		subscriptions.NewRouteWebhook(
			lsProvider,
			subsRepo,
			entService,
		),
	}
	mux := http.NewServeMux()
	hideRouteMiddleware := httptools.Hidden(
		httptools.IsLocalNetworkReq,
		http.StatusNotFound,
	)
	if cfg.Metrics.Enable {
		metricsHandler := metrics.Init(cfg.Metrics.GoMetrics)
		mux.Handle(
			"GET "+cfg.Metrics.Path,
			httptools.Wrap(metricsHandler, hideRouteMiddleware),
		)
	}
	mux.Handle(
		"GET "+healthcheckProbePath,
		httptools.Wrap(
			nil,
			hideRouteMiddleware,
			gracefulshutdown.HealthCheckMiddleware,
			db.HealthCheckMiddleware(database),
		),
	)
	for _, route := range routes {
		route.Register(mux, reflector)
	}
	openapi.NewRoute(reflector).Register(mux, reflector)

	//
	// Middlewares
	//

	// skip tracing, logging and metrics for unnecessary endpoints
	// skip auth for healthz, metrics, and webhook (webhook has its own signature validation)
	middlewares := []func(http.Handler) http.Handler{
		httptools.Skip(tracing.Middleware, healthcheckProbePath, cfg.Metrics.Path),
		httptools.Skip(logger.Middleware, healthcheckProbePath, cfg.Metrics.Path),
		logger.RecoveryMiddleware,
		httptools.Skip(
			auth.Middleware(cfg.Auth.APIKey),
			healthcheckProbePath,
			cfg.Metrics.Path,
			"/v1/webhook/*",
		),
	}
	if cfg.Metrics.Enable {
		middlewares = append(
			middlewares,
			httptools.Skip(
				metrics.Middleware,
				healthcheckProbePath,
				cfg.Metrics.Path,
			),
		)
	}

	//
	// Start server
	//

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := server.New(addr, httptools.Wrap(mux, middlewares...))
	go func() {
		slog.Info("starting server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
	gracefulshutdown.WaitForShutdown(srv)
}
