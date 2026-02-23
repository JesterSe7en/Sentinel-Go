package limiter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

type SentinelEngine struct {
	log               *logger.Logger
	rdb               RedisBackend
	rateLimitConfig   *config.RateLimitConfig
	engineMetrics     *SentinelEngineMetrics
	middlewareMetrics *MiddlewareMetrics
	grpcMetrics       *GRPCMetrics
}

type RedisBackend interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	ExecuteScript(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm, cfg any) (storage.RateLimitResult, error)
	PingRDB(ctx context.Context) error
}

type EngineInterface interface {
	Allow(ctx context.Context, key string) (storage.RateLimitResult, error)
	ListAlgorithm() []string
	GetCurrentAlgorithm(ctx context.Context) (string, error)
	UpdateAlgorithm(ctx context.Context, algo algorithm.RateLimitAlgorithm) error
	GetFailOpen(ctx context.Context) (bool, error)
	SetFailOpen(ctx context.Context, failOpen bool) (bool, error)
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
const failOpenConfigKey = "sentinel:global:failopen"

func newSentinelEngineWithBackend(rdb RedisBackend, cfg *config.SentinelAppConfig, reg prometheus.Registerer) (*SentinelEngine, error) {
	initialAlgo, err := algorithm.ParseAlgorithm(cfg.RateLimitCfg.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("invalid initial algorithm specified in config: %w", err)
	}
	if _, exists := storage.Scripts[initialAlgo.String()]; !exists {
		return nil, fmt.Errorf("unknown limit algo specified: %s", initialAlgo)
	}

	engine := &SentinelEngine{
		rdb:               rdb,
		log:               nil,
		rateLimitConfig:   &cfg.RateLimitCfg,
		engineMetrics:     registerSentinelEngineMetrics(reg),
		middlewareMetrics: registerMiddlewareMetrics(reg),
		grpcMetrics:       registerGRPCMetrics(reg),
	}

	_, err = engine.rdb.SetNX(context.Background(), algorithmConfigKey, initialAlgo.String(), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to set default algorithm: %w", err)
	}

	failOpenStr, err := engine.rdb.Get(context.Background(), failOpenConfigKey)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get fail open config: %w", err)
	}

	// override fail open config if it exists on redis
	failOpen, err := strconv.ParseBool(failOpenStr)
	if err == nil {
		engine.rateLimitConfig.FailOpen = failOpen
	}
	// if not, stick to the env variable setting

	return engine, nil
}

func NewSentinelEngine(rdb RedisBackend, log *logger.Logger, cfg *config.SentinelAppConfig, reg prometheus.Registerer) (*SentinelEngine, error) {
	engine, err := newSentinelEngineWithBackend(rdb, cfg, reg)
	if err != nil {
		return nil, err
	}

	engine.log = log
	return engine, nil
}

func (se *SentinelEngine) Allow(ctx context.Context, key string) (storage.RateLimitResult, error) {
	configTimer := prometheus.NewTimer(se.engineMetrics.sentinelConfigFetchDuration.WithLabelValues("lookup"))
	algo, err := se.GetCurrentAlgorithm(ctx)
	configTimer.ObserveDuration()
	if err != nil {
		se.engineMetrics.sentinelAllowErrorsTotal.WithLabelValues("redis_error").Inc()
		return storage.RateLimitResult{}, err
	}

	changeTo, err := algorithm.ParseAlgorithm(algo)
	if err != nil {
		return storage.RateLimitResult{}, err
	}

	timer := prometheus.NewTimer(se.engineMetrics.sentinelCheckDuration.WithLabelValues(changeTo.String()))

	results, err := se.checkAllow(ctx, key, changeTo)

	timer.ObserveDuration()

	if !results.Allowed {
		decision := "blocked"
		se.engineMetrics.sentinelRequestTotal.WithLabelValues(decision, changeTo.String(), getClientTypeFromKey(key)).Inc()
	} else {
		decision := "allowed"
		se.engineMetrics.sentinelRequestTotal.WithLabelValues(decision, changeTo.String(), getClientTypeFromKey(key)).Inc()
	}

	return results, err
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

func (se *SentinelEngine) checkAllow(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm) (storage.RateLimitResult, error) {
	cfg, err := se.rateLimitConfig.GetConfigForAlgorithm(algo.String())
	if err != nil {
		se.engineMetrics.sentinelAllowErrorsTotal.WithLabelValues("config_error").Inc()
		return storage.RateLimitResult{}, fmt.Errorf("failed to get config for algorithm: %w", err)
	}

	results, err := se.rdb.ExecuteScript(ctx, key, algo, cfg)
	if err != nil {
		return storage.RateLimitResult{}, fmt.Errorf("error running script: %w", err)
	}

	if !results.Allowed {
		se.engineMetrics.sentinelAllowErrorsTotal.WithLabelValues("rate_limit_exceeded").Inc()
	}

	return results, nil
}

func (se *SentinelEngine) GetFailOpen(ctx context.Context) (bool, error) {
	failOpenStr, err := se.rdb.Get(ctx, failOpenConfigKey)
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("failed to get fail open config: %w", err)
	}
	if err != nil {
		return false, fmt.Errorf("failed to get fail open config: %w", err)
	}

	if failOpen, err := strconv.ParseBool(failOpenStr); err != nil {
		return false, fmt.Errorf("failed to parse fail open config: %w", err)
	} else {
		return failOpen, nil
	}
}

func (se *SentinelEngine) SetFailOpen(ctx context.Context, failOpen bool) (bool, error) {
	se.rateLimitConfig.FailOpen = failOpen

	return se.rdb.Set(ctx, failOpenConfigKey, strconv.FormatBool(failOpen), 0)

}

func getClientTypeFromKey(key string) string {
	if len(key) > 7 && key[:7] == "apikey:" {
		return "apikey"
	}
	return "ip"
}
