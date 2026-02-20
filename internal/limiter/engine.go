package limiter

import (
	"context"
	"fmt"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type SentinelEngine struct {
	log               *logger.Logger
	rdb               *storage.RedisStorage
	rateLimitConfig   *config.RateLimitConfig
	engineMetrics     *SentinelEngineMetrics
	middlewareMetrics *MiddlewareMetrics
	grpcMetrics       *GRPCMetrics
}

type SentinelEngineMetrics struct {
	sentinelRequestTotal        *prometheus.CounterVec
	sentinelCheckDuration       *prometheus.HistogramVec
	sentinelAlgorithmSwitches   *prometheus.CounterVec
	sentinelAlgorithmInUse      *prometheus.GaugeVec
	sentinelAllowErrorsTotal    *prometheus.CounterVec
	sentinelConfigFetchDuration *prometheus.HistogramVec
}

func registerSentinelEngineMetrics(reg prometheus.Registerer) *SentinelEngineMetrics {
	return &SentinelEngineMetrics{
		sentinelRequestTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "sentinel_requests_total",
			Help: "help!",
		}, []string{"decision", "algorithm", "client_type"}),
		sentinelCheckDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sentinel_check_duration_seconds",
			Help:    "help!",
			Buckets: []float64{0.001, 0.01, 0.1, 1, 10},
		}, []string{"algorithm"}),
		sentinelAlgorithmSwitches: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "sentinel_algorithm_switches_total",
			Help: "help!",
		}, []string{"from", "to"}),
		sentinelAlgorithmInUse: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "sentinel_algorithm_in_use",
			Help: "help!",
		}, []string{"algorithm"}),
		sentinelAllowErrorsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "sentinel_allow_errors_total",
			Help: "help!",
		}, []string{"error_type"}),
		sentinelConfigFetchDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sentinel_config_fetch_duration_seconds",
			Help:    "help!",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
		}, []string{"algorithm"}),
	}
}

const algorithmConfigKey = "sentinel:global:algorithm"

func NewSentinelEngine(rdb *storage.RedisStorage, log *logger.Logger, cfg *config.SentinelAppConfig, reg prometheus.Registerer) (*SentinelEngine, error) {
	initialAlgo, err := algorithm.ParseAlgorithm(cfg.RateLimitCfg.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("invalid initial algorithm specified in config: %w", err)
	}
	if _, exists := storage.Scripts[initialAlgo.String()]; !exists {
		return nil, fmt.Errorf("unknown limit algo specified: %s", initialAlgo)
	}

	engine := &SentinelEngine{
		rdb:               rdb,
		log:               log,
		rateLimitConfig:   &cfg.RateLimitCfg,
		engineMetrics:     registerSentinelEngineMetrics(reg),
		middlewareMetrics: registerMiddlewareMetrics(reg),
		grpcMetrics:       registerGRPCMetrics(reg),
	}

	_, err = engine.rdb.SetNX(context.Background(), algorithmConfigKey, initialAlgo.String(), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to set default algorithm: %w", err)
	}

	return engine, nil
}

func (se *SentinelEngine) Allow(ctx context.Context, key string) (bool, error) {
	configTimer := prometheus.NewTimer(se.engineMetrics.sentinelConfigFetchDuration.WithLabelValues("lookup"))
	algo, err := se.GetCurrentAlgorithm(ctx)
	configTimer.ObserveDuration()
	if err != nil {
		se.engineMetrics.sentinelAllowErrorsTotal.WithLabelValues("redis_error").Inc()
		return false, err
	}

	changeTo, err := algorithm.ParseAlgorithm(algo)
	if err != nil {
		return false, err
	}

	timer := prometheus.NewTimer(se.engineMetrics.sentinelCheckDuration.WithLabelValues(changeTo.String()))

	success, err := se.checkAllow(ctx, key, changeTo)

	timer.ObserveDuration()

	if !success {
		decision := "blocked"
		se.engineMetrics.sentinelRequestTotal.WithLabelValues(decision, changeTo.String(), "FIX THIS").Inc()
	} else {
		decision := "allowed"
		se.engineMetrics.sentinelRequestTotal.WithLabelValues(decision, changeTo.String(), "FIX THIS").Inc()
	}

	return success, err
}

func (se *SentinelEngine) ListAlgorithm() []string {
	return storage.GetAlgorithmNames()
}

func (se *SentinelEngine) GetCurrentAlgorithm(ctx context.Context) (string, error) {
	algo, err := se.rdb.Get(ctx, algorithmConfigKey)
	if err != nil {
		return "", fmt.Errorf("failed to get current algorithm: %w", err)
	}
	return algo, nil
}

func (se *SentinelEngine) UpdateAlgorithm(ctx context.Context, algo algorithm.RateLimitAlgorithm) error {
	if !algo.IsValid() {
		return fmt.Errorf("invalid algorithm specified: %s", algo)
	}

	if _, exists := storage.Scripts[algo.String()]; !exists {
		return fmt.Errorf("unknown limit algo specified: %s", algo)
	}

	currentAlgo, _ := se.GetCurrentAlgorithm(ctx)

	_, err := se.rdb.Set(ctx, algorithmConfigKey, algo.String(), 0)
	if err != nil {
		return fmt.Errorf("failed to update algorithm: %w", err)
	}

	se.engineMetrics.sentinelAlgorithmSwitches.WithLabelValues(currentAlgo, algo.String()).Inc()
	se.engineMetrics.sentinelAlgorithmInUse.WithLabelValues(algo.String()).Set(1)

	return nil
}

func (se *SentinelEngine) checkAllow(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm) (bool, error) {
	cfg, err := se.rateLimitConfig.GetConfigForAlgorithm(algo.String())
	if err != nil {
		se.engineMetrics.sentinelAllowErrorsTotal.WithLabelValues("config_error").Inc()
		return false, fmt.Errorf("failed to get config for algorithm: %w", err)
	}

	success, err := se.rdb.ExecuteScript(ctx, key, algo, cfg)
	if err != nil {
		return false, fmt.Errorf("error running script: %w", err)
	}

	return success, nil
}
