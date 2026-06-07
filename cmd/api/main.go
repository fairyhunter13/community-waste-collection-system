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

	"github.com/labstack/echo/v4"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
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

	// Tracing
	ctx := context.Background()
	_, shutdownTracer, err := observability.InitTracer(ctx, cfg)
	if err != nil {
		logger.Error("init tracer", "error", err)
		// non-fatal: continue without tracing
	}

	// Database
	db, err := repository.Connect(cfg)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// Storage
	s3, err := storage.NewS3Client(cfg)
	if err != nil {
		logger.Error("init S3 client", "error", err)
		os.Exit(1)
	}
	if err := s3.EnsureBucket(ctx); err != nil {
		logger.Error("ensure S3 bucket", "error", err)
		os.Exit(1)
	}

	// Repositories
	householdRepo := repository.NewHouseholdRepository(db)
	pickupRepo := repository.NewPickupRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)

	// Services
	householdSvc := service.NewHouseholdService(householdRepo)
	pickupSvc := service.NewPickupService(pickupRepo, paymentRepo, db)
	paymentSvc := service.NewPaymentService(paymentRepo, s3)
	reportSvc := service.NewReportService(paymentRepo)

	// HTTP server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.RecoverMiddleware(logger))
	e.Use(middleware.RequestLogger(logger))
	e.Use(middleware.OtelTrace(cfg.OTELServiceName))
	e.Use(middleware.RequestMetrics())

	h := handler.New(householdSvc, pickupSvc, paymentSvc, reportSvc, cfg, db)
	h.RegisterRoutes(e)

	// Background worker
	workerCtx, workerCancel := context.WithCancel(context.Background())
	canceler := worker.NewOrganicCanceler(pickupRepo, logger, cfg)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		canceler.Start(workerCtx)
	}()

	// pprof debug server
	go func() {
		debugAddr := ":" + cfg.DebugPort
		logger.Info("starting pprof server", "addr", debugAddr)
		if err := http.ListenAndServe(debugAddr, nil); err != nil && err != http.ErrServerClosed { //nolint:gosec
			logger.Error("pprof server error", "error", err)
		}
	}()

	// Prometheus metrics server
	metricsAddr := ":" + cfg.MetricsPort
	metricsSrv, err := observability.StartMetricsServer(metricsAddr)
	if err != nil {
		logger.Error("start metrics server", "error", err)
	} else {
		logger.Info("metrics server started", "addr", metricsAddr)
	}

	// Start API server
	go func() {
		addr := ":" + cfg.AppPort
		logger.Info("starting API server", "addr", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
		}
	}()

	// Graceful shutdown on signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down")

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown", "error", err)
	}

	workerCancel()
	wg.Wait()

	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics server shutdown", "error", err)
		}
	}

	if shutdownTracer != nil {
		if err := shutdownTracer(shutdownCtx); err != nil {
			logger.Error("tracer shutdown", "error", err)
		}
	}

	logger.Info("shutdown complete")
}
