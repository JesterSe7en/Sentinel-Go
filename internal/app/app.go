package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/api/v1/pb"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/limiter"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"google.golang.org/grpc"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// App holds the application's dependencies and configuration.
type App struct {
	Log        *logger.Logger
	engine     *limiter.SentinelEngine
	storage    *storage.RedisStorage
	httpServer *http.Server
	grpcServer *grpc.Server
	reg        *prometheus.Registry
}

// New creates and returns a new app instance.
func New(sCfg *config.SentinelAppConfig) (*App, error) {
	log, err := logger.New("sentinel.log", false, true)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	rc := sCfg.RedisCfg
	rdb := storage.NewRedisStorage(rc.MasterName, rc.SentinelAddrs, rc.Password, rc.DB, reg)
	if rdb == nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	engine, err := limiter.NewSentinelEngine(rdb, log, sCfg, reg)
	if err != nil {
		return nil, errors.New("failed to start Sentinel engine")
	}

	return &App{
		Log:     log,
		engine:  engine,
		storage: rdb,
		reg:     reg,
	}, nil
}

func (a *App) Run() error {
	defer a.Log.Sync()

	if err := a.initGRPC(); err != nil {
		return fmt.Errorf("failed to initialize gRPC: %w", err)
	}

	grpcLis, err := net.Listen("tcp", ":50051")
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}

	go func() {
		a.Log.Info("starting_grpc_server", "address", ":50051")
		if err := a.grpcServer.Serve(grpcLis); err != nil {
			a.Log.Error("grpc_server_error", "error", err)
		}
	}()

	mux := http.NewServeMux()

	mockAPI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Success! You have reached the protected API."))
	})

	mux.Handle("/data", a.engine.RateLimitMiddleware(mockAPI))

	a.httpServer = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	serverErrors := make(chan error, 1)

	mux.Handle("/metrics", promhttp.HandlerFor(a.reg, promhttp.HandlerOpts{}))

	go func() {
		a.Log.Info("starting_http_server", "address", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-sigChan:
		a.Log.Info("shutdown_signal_received", "signal", sig)

		shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownRelease()

		a.grpcServer.GracefulStop()

		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		a.Log.Info("graceful_shutdown_complete")
	}

	return nil
}

func (a *App) initGRPC() error {
	handler := limiter.NewGRPCHandler(a.engine)
	a.grpcServer = grpc.NewServer()
	pb.RegisterServiceRateLimiterServer(a.grpcServer, handler)
	return nil
}
