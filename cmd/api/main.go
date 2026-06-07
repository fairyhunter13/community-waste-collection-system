// Package main is the entry point for the community waste collection API server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // register pprof handlers on DefaultServeMux
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/handler"
	"github.com/fairyhunter13/community-waste-collection-system/internal/middleware"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
	"github.com/fairyhunter13/community-waste-collection-system/internal/service"
	"github.com/fairyhunter13/community-waste-collection-system/internal/storage"
	"github.com/fairyhunter13/community-waste-collection-system/internal/worker"
)

func main() {
	cfg := config.Load()
	logger := observability.NewLogger(cfg)
	slog.SetDefault(logger)

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	_, shutdownTracer, err := observability.InitTracer(ctx, cfg)
	if err != nil {
		logger.Error("init tracer", "error", err)
		// non-fatal: continue without tracing
	}

	db, s3 := mustInitInfra(ctx, cfg, logger)
	defer func() { _ = db.Close() }()

	e := buildHTTPServer(cfg, logger, db, s3)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	wg := startBackgroundWorker(workerCtx, cfg, repository.NewPickupRepository(db), logger)

	startDebugServer(cfg, logger)
	metricsSrv := startMetricsServer(cfg, logger)
	startAPIServer(e, cfg, logger)

	awaitShutdown()
	gracefulShutdown(e, metricsSrv, shutdownTracer, workerCancel, wg, cfg, logger)
}

// mustInitInfra connects the DB and S3 storage. Exits the process on failure;
// at startup these are non-recoverable.
func mustInitInfra(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*sqlx.DB, *storage.S3Client) {
	db, err := repository.Connect(cfg)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	s3, err := storage.NewS3Client(cfg) //nolint:contextcheck // signature does not accept ctx
	if err != nil {
		logger.Error("init S3 client", "error", err)
		os.Exit(1)
	}
	if err := s3.EnsureBucket(ctx); err != nil {
		logger.Error("ensure S3 bucket", "error", err)
		os.Exit(1)
	}
	return db, s3
}

// buildHTTPServer wires repositories, services, handlers, and middleware into
// an Echo instance ready to start.
func buildHTTPServer(cfg *config.Config, logger *slog.Logger, db *sqlx.DB, s3 *storage.S3Client) *echo.Echo {
	householdRepo := repository.NewHouseholdRepository(db)
	pickupRepo := repository.NewPickupRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)

	householdSvc := service.NewHouseholdService(householdRepo)
	pickupSvc := service.NewPickupService(pickupRepo, paymentRepo, db)
	paymentSvc := service.NewPaymentService(paymentRepo, pickupRepo, s3)
	reportSvc := service.NewReportService(paymentRepo)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.RecoverMiddleware(logger))
	e.Use(middleware.RequestLogger(logger))
	e.Use(middleware.OtelTrace(cfg.OTELServiceName))
	e.Use(middleware.RequestMetrics())

	h := handler.New(householdSvc, pickupSvc, paymentSvc, reportSvc, cfg, db)
	h.RegisterRoutes(e)
	return e
}

func startBackgroundWorker(ctx context.Context, cfg *config.Config, pickupRepo domain.PickupRepository, logger *slog.Logger) *sync.WaitGroup {
	canceler := worker.NewOrganicCanceler(pickupRepo, logger, cfg)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		canceler.Start(ctx)
	}()
	return &wg
}

func startDebugServer(cfg *config.Config, logger *slog.Logger) {
	go func() {
		debugAddr := ":" + cfg.DebugPort
		logger.Info("starting pprof server", "addr", debugAddr)
		if err := http.ListenAndServe(debugAddr, nil); err != nil && err != http.ErrServerClosed { //nolint:gosec
			logger.Error("pprof server error", "error", err)
		}
	}()
}

func startMetricsServer(cfg *config.Config, logger *slog.Logger) *http.Server {
	metricsAddr := ":" + cfg.MetricsPort
	metricsSrv, err := observability.StartMetricsServer(metricsAddr)
	if err != nil {
		logger.Error("start metrics server", "error", err)
		return nil
	}
	logger.Info("metrics server started", "addr", metricsAddr)
	return metricsSrv
}

func startAPIServer(e *echo.Echo, cfg *config.Config, logger *slog.Logger) {
	e.Server.ReadHeaderTimeout = cfg.HTTPReadHeaderTimeout
	e.Server.ReadTimeout = cfg.HTTPReadTimeout
	e.Server.WriteTimeout = cfg.HTTPWriteTimeout
	e.Server.IdleTimeout = cfg.HTTPIdleTimeout

	go func() {
		addr := ":" + cfg.AppPort
		logger.Info("starting API server",
			"addr", addr,
			"read_header_timeout", cfg.HTTPReadHeaderTimeout,
			"read_timeout", cfg.HTTPReadTimeout,
			"write_timeout", cfg.HTTPWriteTimeout,
			"idle_timeout", cfg.HTTPIdleTimeout,
		)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
		}
	}()
}

func awaitShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func gracefulShutdown(
	e *echo.Echo,
	metricsSrv *http.Server,
	shutdownTracer func(context.Context) error,
	workerCancel context.CancelFunc,
	wg *sync.WaitGroup,
	cfg *config.Config,
	logger *slog.Logger,
) {
	logger.Info("shutting down")

	// Background context: shutdown must not inherit a cancelled upstream context.
	httpShutdownCtx, httpShutdownRelease := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout) //nolint:contextcheck
	defer httpShutdownRelease()

	if err := e.Shutdown(httpShutdownCtx); err != nil {
		logger.Error("HTTP server shutdown", "error", err)
	}

	workerCancel()
	workerDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workerDone)
	}()
	select {
	case <-workerDone:
		logger.Info("background worker drained")
	case <-time.After(cfg.WorkerShutdownTimeout):
		logger.Error("worker shutdown timed out — abandoning", "timeout", cfg.WorkerShutdownTimeout)
	}

	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(httpShutdownCtx); err != nil {
			logger.Error("metrics server shutdown", "error", err)
		}
	}
	if shutdownTracer != nil {
		if err := shutdownTracer(httpShutdownCtx); err != nil {
			logger.Error("tracer shutdown", "error", err)
		}
	}

	logger.Info("shutdown complete")
}
