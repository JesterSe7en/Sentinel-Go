package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JesterSe7en/Sentinel-Go/api/v1/pb"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/limiter"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

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
	appCfg     *config.SentinelAppConfig
}

// New creates and returns a new app instance.
func New(sCfg *config.SentinelAppConfig) (*App, error) {
	log, err := logger.New("", false, true)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	rc := sCfg.RedisCfg
	rdb, err := storage.NewRedisStorage(rc.MasterName, rc.SentinelAddrs, rc.Password, rc.DB, reg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Info("redis_connected", "master", rc.MasterName, "sentinels", rc.SentinelAddrs)

	engine, err := limiter.NewSentinelEngine(rdb, log, sCfg, reg)
	if err != nil {
		return nil, errors.New("failed to start Sentinel engine")
	}

	return &App{
		Log:     log,
		engine:  engine,
		storage: rdb,
		reg:     reg,
		appCfg:  sCfg,
	}, nil
}

func (a *App) Run() error {
	defer a.Log.Sync()

	// ------ Initilize gRPC server ------------
	if err := a.initGRPC(); err != nil {
		return fmt.Errorf("failed to initialize gRPC: %w", err)
	}

	grpcLis, err := net.Listen("tcp", ":"+a.appCfg.ServerCfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}

	go func() {
		a.Log.Info("starting_grpc_server", "address", ":"+a.appCfg.ServerCfg.GRPCPort)
		if err := a.grpcServer.Serve(grpcLis); err != nil {
			a.Log.Error("grpc_server_error", "error", err)
		}
	}()

	// ------ Initilize http server ------------
	mux := http.NewServeMux()

	mockAPI := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Success! You have reached the protected API."))
	})

	mux.Handle("/data", a.engine.RateLimitMiddleware(mockAPI))
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	mux.Handle("/ready", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.engine.PingRDB(context.Background()); err != nil {
			a.Log.Error("readiness_check_failed", "error", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	}))

	a.httpServer = &http.Server{
		Addr:    ":" + a.appCfg.ServerCfg.HTTPPort,
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

		shutdownCtx, shutdownRelease := context.WithTimeout(
			context.Background(), a.appCfg.BootstrapCfg.ShutdownTimeout)
		defer shutdownRelease()

		a.Log.Info("grpc_graceful_stop_start")
		a.grpcServer.GracefulStop()
		a.Log.Info("grpc_graceful_stop_complete")

		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		a.Log.Info("graceful_shutdown_complete")
	}

	return nil
}

func (a *App) initGRPC() error {
	handler := limiter.NewGRPCHandler(a.engine)
	cred, err := a.loadTLSCredentials()
	if err != nil {
		return fmt.Errorf("failed to load TLS credentials: %w", err)
	}
	a.Log.Info("tls_credentials_loaded", "ca", a.appCfg.CertCfg.CertCAPath, "cert", a.appCfg.CertCfg.CertServerCRTPath)
	a.grpcServer = grpc.NewServer(grpc.Creds(cred))
	pb.RegisterRateLimiterServiceServer(a.grpcServer, handler)
	return nil
}

func (a *App) loadTLSCredentials() (credentials.TransportCredentials, error) {
	// sources: https://grpc.io/docs/guides/auth/
	// https://github.com/grpc/grpc-go/blob/master/examples/features/encryption/mTLS/server/main.go
	// https://github.com/grpc/grpc-go/blob/master/Documentation/grpc-auth-support.md
	// "If certificates to establish the identity of the client need to be included in the
	// credentials (eg: for mTLS), use NewTLS instead, where a complete tls.Config can be specified."

	// get ca

	certCfg := a.appCfg.CertCfg

	caCert, err := os.ReadFile(certCfg.CertCAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// x509 = a standardized digital document (defined by ITU-T) that acts as a
	// "digital passport," binding a public key to an identity (user, server, or device)
	// to enable secure, trusted communication.
	caCertPool := x509.NewCertPool()
	// PEM is the encoding format for the .crt file
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	serverCert, err := tls.LoadX509KeyPair(certCfg.CertServerCRTPath, certCfg.CertSeverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	return credentials.NewTLS(tlsConfig), nil
}
