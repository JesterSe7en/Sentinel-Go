package storage

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

type RedisMetrics struct {
	rateLimiterDecisionTotal *prometheus.CounterVec
	operationDuration        *prometheus.HistogramVec
	operationError           *prometheus.CounterVec
	connectionPoolIdle       *prometheus.GaugeVec
	connectionPoolInUse      *prometheus.GaugeVec
	rateLimiterLatency       *prometheus.HistogramVec
	poolWaitDuration         *prometheus.HistogramVec
}

func registerRedisMetrics(reg prometheus.Registerer) *RedisMetrics {
	m := &RedisMetrics{
		rateLimiterDecisionTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "rate_limiter_decision_total",
			Help: "Allowed/rejected count by rate limiting algorithm",
		}, []string{"key", "decision", "algorithm"}),
		operationDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Name:    "redis_operation_duration_seconds",
			Help:    "Latency by operation (get/set/script) + status in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"operation", "status"}),
		operationError: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "redis_operation_errors_total",
			Help: "Errors by operation + error type",
		}, []string{"operation", "error_type"}),
		connectionPoolIdle: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "redis_connection_pool_idle",
			Help: "Number of idle connections in the pool",
		}, []string{"pool"}),
		connectionPoolInUse: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "redis_connection_pool_in_use",
			Help: "Number of in use connections in the pool",
		}, []string{"pool"}),
		rateLimiterLatency: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Name:    "rate_limiter_execution_duration_seconds",
			Help:    "End-to-end latency of ExecuteScript by algorithm in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"algorithm"}),
		poolWaitDuration: promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
			Name:    "redis_connection_pool_wait_seconds",
			Help:    "Time waiting for available connection",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .05, .1},
		}, []string{"pool"}),
	}
	return m
}

// Basically we are embedding the raw binary into the go project
// Use the embed directive to pull in all .lua files in the lua folder
//
//go:embed algos/*.lua
var luaScripts embed.FS

var Scripts = make(map[string]*redis.Script)

var algoRegistry = map[string]string{
	"TokenBucket":          "algos/token_bucket.lua",
	"LeakyBucket":          "algos/leaky_bucket.lua",
	"FixedWindow":          "algos/fixed_window.lua",
	"SlidingWindowLog":     "algos/sliding_window_log.lua",
	"SlidingWindowCounter": "algos/sliding_window_counter.lua",
}

var (
	TokenBucketScript          *redis.Script
	LeakyBucketScript          *redis.Script
	FixedWindowScript          *redis.Script
	SlidingWindowLogScript     *redis.Script
	SlidingWindowCounterScript *redis.Script
)

type RedisStorage struct {
	rdb     *redis.Client
	metrics *RedisMetrics
	stopCh  chan struct{}
}

type RateLimitResult struct {
	Allowed   bool
	Limit     int
	Remaining int
	Reset     int
}

// gets executed before everything in this package - init is "special"
func init() {
	for algoName, scriptPath := range algoRegistry {
		src, err := luaScripts.ReadFile(scriptPath)
		if err != nil {
			panic(fmt.Sprintf("failed to read embedded script %s: %v", scriptPath, err))
		}
		Scripts[algoName] = redis.NewScript(string(src))
	}
}

func getErrorType(err error) string {
	if err == nil {
		return "none"
	}
	switch err {
	case redis.Nil:
		return "not_found"
	default:
		return "unknown"
	}
}

func NewRedisStorage(masterName string, sentinels []string, password string, db int, reg prometheus.Registerer) *RedisStorage {
	rdb := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    masterName,
		SentinelAddrs: sentinels,
		Password:      password,
		DB:            db,
		MaxRetries:    3,
		MinIdleConns:  5,
	})

	status := rdb.Ping(context.Background())

	if err := status.Err(); err != nil {
		panic(fmt.Sprintf("failed to connect to redis cluster: %v", err))
	}

	m := registerRedisMetrics(reg)

	storage := &RedisStorage{
		rdb:     rdb,
		metrics: m,
		stopCh:  make(chan struct{}),
	}

	go storage.collectPoolMetrics()

	return storage
}

func (rs *RedisStorage) Close() {
	close(rs.stopCh)
}

func (rs *RedisStorage) Get(ctx context.Context, key string) (string, error) {
	start := time.Now()
	ret, err := rs.rdb.Get(ctx, key).Result()
	duration := time.Since(start).Seconds()
	if err != nil {
		rs.metrics.operationError.WithLabelValues("Get", getErrorType(err)).Inc()
		rs.metrics.operationDuration.WithLabelValues("Get", "error").Observe(duration)
		return "", err
	}

	rs.metrics.operationDuration.WithLabelValues("Get", "success").Observe(duration)
	return ret, err
}

func (rs *RedisStorage) Set(ctx context.Context, key string, value any, duration time.Duration) (bool, error) {
	start := time.Now()
	_, err := rs.rdb.Set(ctx, key, value, duration).Result()
	setDuration := time.Since(start).Seconds()

	if err != nil {
		rs.metrics.operationError.WithLabelValues("Set", getErrorType(err)).Inc()
		rs.metrics.operationDuration.WithLabelValues("Set", "error").Observe(setDuration)
		return false, err
	}

	rs.metrics.operationDuration.WithLabelValues("Set", "success").Observe(setDuration)
	return true, nil
}

func (rs *RedisStorage) SetNX(ctx context.Context, key string, value any, duration time.Duration) (bool, error) {
	start := time.Now()
	_, err := rs.rdb.SetNX(ctx, key, value, duration).Result()
	setNXDuration := time.Since(start).Seconds()

	if err != nil {
		if err == redis.Nil {
			rs.metrics.operationDuration.WithLabelValues("SetNX", "success").Observe(setNXDuration)
			return false, nil
		}
		rs.metrics.operationDuration.WithLabelValues("SetNX", "error").Observe(setNXDuration)
		rs.metrics.operationError.WithLabelValues("SetNX", getErrorType(err)).Inc()
		return false, err
	}
	rs.metrics.operationDuration.WithLabelValues("SetNX", "success").Observe(setNXDuration)
	return true, nil
}

func (rs *RedisStorage) collectPoolMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Track previous WaitDurationNs to compute the delta each tick,
	// so we observe the incremental wait time rather than the cumulative total.
	var prevWaitNs int64

	for {
		select {
		case <-rs.stopCh:
			return
		case <-ticker.C:
			poolStats := rs.rdb.PoolStats()
			if poolStats != nil {
				rs.metrics.connectionPoolIdle.WithLabelValues("default").Set(float64(poolStats.IdleConns))
				rs.metrics.connectionPoolInUse.WithLabelValues("default").Set(float64(poolStats.TotalConns - poolStats.IdleConns))

				// WaitDurationNs is a cumulative int64 nanosecond counter.
				// Observe only the delta since the last tick so the histogram
				// reflects per-interval wait time, not an ever-growing total.
				deltaNs := poolStats.WaitDurationNs - prevWaitNs
				if deltaNs > 0 {
					rs.metrics.poolWaitDuration.WithLabelValues("default").Observe(float64(deltaNs) / 1e9)
				}
				prevWaitNs = poolStats.WaitDurationNs
			}
		}
	}
}

func (rs *RedisStorage) ExecuteScript(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm, algoConfig any) (RateLimitResult, error) {
	if key == "" {
		return RateLimitResult{}, fmt.Errorf("key is nil")
	}
	if !algo.IsValid() {
		return RateLimitResult{}, fmt.Errorf("unknown algorithm %s", algo.String())
	}

	algoStr := algo.String()

	latencyTimer := prometheus.NewTimer(rs.metrics.rateLimiterLatency.WithLabelValues(algoStr))
	defer latencyTimer.ObserveDuration()

	scriptStart := time.Now()

	scriptToRun := Scripts[algoStr]
	args := getArgsFromConfig(algoConfig)

	results, err := scriptToRun.Run(ctx, rs.rdb, []string{key}, args).Int64Slice()

	// format will always be {allowed, limit, remaining, reset}
	// matching gRPC response format
	allowed, limit, remaining, reset := results[0], results[1], results[2], results[3]

	scriptDuration := time.Since(scriptStart).Seconds()
	if err != nil {
		rs.metrics.operationDuration.WithLabelValues("script", "error").Observe(scriptDuration)
		rs.metrics.operationError.WithLabelValues("script", getErrorType(err)).Inc()
		rs.metrics.rateLimiterDecisionTotal.WithLabelValues(key, "error", algoStr).Inc()
		return RateLimitResult{}, err
	}
	rs.metrics.operationDuration.WithLabelValues("script", "success").Observe(scriptDuration)

	decision := "allowed"
	if allowed == 0 {
		decision = "rejected"
	}
	rs.metrics.rateLimiterDecisionTotal.WithLabelValues(key, decision, algoStr).Inc()

	return RateLimitResult{
		Allowed:   allowed == 1,
		Limit:     int(limit),
		Remaining: int(remaining),
		Reset:     int(reset),
	}, nil
}

func GetAlgorithmNames() []string {
	return []string{
		"TokenBucket",
		"LeakyBucket",
		"FixedWindow",
		"SlidingWindowLog",
		"SlidingWindowCounter",
	}
}

func getArgsFromConfig(cfg any) []any {
	if cfg == nil {
		return nil
	}

	now := time.Now().Unix()

	switch c := cfg.(type) {
	case *config.TokenBucketConfig:
		return []any{c.Capacity, c.RefillRate, now}
	case *config.LeakyBucketConfig:
		return []any{c.Capacity, c.LeakRate, now}
	case *config.FixedWindowConfig:
		windowSecs := int(c.Window.Seconds())
		return []any{c.Limit, windowSecs}
	case *config.SlidingWindowLogConfig:
		return []any{c.Limit, c.SlidingWindowSize, now}
	case *config.SlidingWindowCounterConfig:
		return []any{c.Limit, c.SlidingWindowSize, c.BucketSize, now}
	default:
		return nil
	}
}
