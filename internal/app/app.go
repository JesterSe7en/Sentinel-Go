package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/limiter"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
)

// App holds the application's dependencies and configuration.
type App struct {
	Log    *logger.Logger
	engine *limiter.SentinelEngine
	server *http.Server
}

// New creates and returns a new app instance.
func New(sCfg *config.SentinelAppConfig) (*App, error) {
	log, err := logger.New("sentinel.log", false, true)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	rc := sCfg.RedisCfg
	rdb := storage.NewRedisClient(rc.MasterName, rc.SentinelAddrs, rc.Password, rc.DB)
	if rdb == nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	engine, err := limiter.NewSentinelEngine(rdb, log, sCfg)
	if err != nil {
		return nil, errors.New("failed to start Sentinel engine")
	}

	return &App{
		Log:    log,
		engine: engine,
	}, nil
}

func (a *App) Run() error {
	defer a.Log.Sync()

	mux := http.NewServeMux()

	mockAPI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Success! You have reached the protected API."))
	})

	mux.Handle("/data", a.engine.RateLimitMiddleware(mockAPI))

	a.server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	serverErrors := make(chan error, 1)

	go func() {
		a.Log.Info("starting_server", "address", a.server.Addr)
		if err := a.server.ListenAndServe(); err != http.ErrServerClosed {
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

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		a.Log.Info("graceful_shutdown_complete")
	}

	return nil
}
