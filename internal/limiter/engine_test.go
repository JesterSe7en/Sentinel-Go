package limiter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

type mockRedisStorage struct {
	getFn   func(ctx context.Context, key string) (string, error)
	setFn   func(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	setNXFn func(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	execFn  func(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm, cfg any) (storage.RateLimitResult, error)
	pingFn  func(ctx context.Context) error

	// spies
	setCalls []setCall
	getCalls []string
}

type setCall struct {
	key   string
	value any
}

func (m *mockRedisStorage) Get(ctx context.Context, key string) (string, error) {
	m.getCalls = append(m.getCalls, key)
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	return "", redis.Nil
}

func (m *mockRedisStorage) Set(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	m.setCalls = append(m.setCalls, setCall{key: key, value: value})
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	return true, nil
}

func (m *mockRedisStorage) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	m.setCalls = append(m.setCalls, setCall{key: key, value: value})
	if m.setNXFn != nil {
		return m.setNXFn(ctx, key, value, ttl)
	}
	return true, nil
}

func (m *mockRedisStorage) ExecuteScript(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm, cfg any) (storage.RateLimitResult, error) {
	if m.execFn != nil {
		return m.execFn(ctx, key, algo, cfg)
	}
	return storage.RateLimitResult{Allowed: true}, nil
}

func (m *mockRedisStorage) PingRDB(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func newTestRegistry() prometheus.Registerer {
	return prometheus.NewRegistry()
}

func baseConfig() *config.SentinelAppConfig {
	return &config.SentinelAppConfig{
		RateLimitCfg: config.RateLimitConfig{
			Algorithm: "TokenBucket",
			FailOpen:  false,
			TokenBucket: &config.TokenBucketConfig{
				RefillRate: 1,
				Capacity:   5,
			},
			LeakyBucket: &config.LeakyBucketConfig{
				Capacity: 100,
				LeakRate: 10.0,
			},
			FixedWindow: &config.FixedWindowConfig{
				Limit:  100,
				Window: time.Minute,
			},
			SlidingWindowLog: &config.SlidingWindowLogConfig{
				Limit:             100,
				SlidingWindowSize: 60,
			},
			SlidingWindowCounter: &config.SlidingWindowCounterConfig{
				Limit:             100,
				SlidingWindowSize: 60,
				BucketSize:        10,
			},
		},
	}
}

func newTestEngine(t *testing.T, rdb *mockRedisStorage) *SentinelEngine {
	t.Helper()
	cfg := baseConfig()
	return &SentinelEngine{
		rdb:               rdb,
		rateLimitConfig:   &cfg.RateLimitCfg,
		engineMetrics:     registerSentinelEngineMetrics(newTestRegistry()),
		middlewareMetrics: registerMiddlewareMetrics(newTestRegistry()),
		grpcMetrics:       registerGRPCMetrics(newTestRegistry()),
	}
}

func TestNewSentinelEngine_Success(t *testing.T) {
	rdb := &mockRedisStorage{
		setNXFn: func(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
			return true, nil
		},
		getFn: func(_ context.Context, key string) (string, error) {
			return "", redis.Nil
		},
	}

	engine, err := newSentinelEngineWithBackend(rdb, baseConfig(), newTestRegistry())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestNewSentinelEngine_InvalidAlgorithmInConfig(t *testing.T) {
	cfg := baseConfig()
	cfg.RateLimitCfg.Algorithm = "NotAnAlgorithm"

	tests := []struct {
		name     string
		algoName string
		wantErr  bool
	}{
		{
			name:     "invalid algorithm",
			algoName: "NotAnAlgorithm",
			wantErr:  true,
		},
		{
			name:     "invalid algorithm - (numbers and symbols)",
			algoName: "1234567890!@#$%^&*()",
			wantErr:  true,
		},
		{
			name:     "invalid algorithm - (empty string)",
			algoName: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.RateLimitCfg.Algorithm = tt.algoName
			engine, err := newSentinelEngineWithBackend(&mockRedisStorage{}, cfg, newTestRegistry())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for invalid algorithm, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if engine == nil {
					t.Fatal("expected non-nil engine")
				}
			}
		})
	}
}

func TestNewSentinelEngine_SetNXFailure(t *testing.T) {
	rdb := &mockRedisStorage{
		setNXFn: func(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
			return false, errors.New("redis unavailable")
		},
	}

	_, err := newSentinelEngineWithBackend(rdb, baseConfig(), newTestRegistry())
	if err == nil {
		t.Fatal("expected error when SetNX fails, got nil")
	}
}

func TestNewSentinelEngine_FailOpenOverriddenFromRedis(t *testing.T) {
	rdb := &mockRedisStorage{
		setNXFn: func(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
			return true, nil
		},
		getFn: func(_ context.Context, key string) (string, error) {
			if key == failOpenConfigKey {
				return "true", nil // Redis says fail open = true
			}
			return "", redis.Nil
		},
	}

	cfg := baseConfig()
	cfg.RateLimitCfg.FailOpen = false // env var says false

	engine, err := newSentinelEngineWithBackend(rdb, cfg, newTestRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !engine.rateLimitConfig.FailOpen {
		t.Error("FailOpen should be true (Redis override), got false")
	}
}

func TestNewSentinelEngine_FailOpenKeptFromEnvWhenNoRedisOverride(t *testing.T) {
	rdb := &mockRedisStorage{
		setNXFn: func(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
			return true, nil
		},
		getFn: func(_ context.Context, key string) (string, error) {
			return "", redis.Nil
		},
	}

	cfg := baseConfig()
	cfg.RateLimitCfg.FailOpen = true // pretent env var says true

	engine, err := newSentinelEngineWithBackend(rdb, cfg, newTestRegistry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !engine.rateLimitConfig.FailOpen {
		t.Error("FailOpen should remain true (env var, no Redis override), got false")
	}
}

func TestAllow_RequestAllowed(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "TokenBucket", nil
		},
		execFn: func(_ context.Context, _ string, _ algorithm.RateLimitAlgorithm, _ any) (storage.RateLimitResult, error) {
			return storage.RateLimitResult{Allowed: true, Remaining: 4}, nil
		},
	}

	engine := newTestEngine(t, rdb)
	result, err := engine.Allow(context.Background(), "ip:192.168.1.1")
	if err != nil {
		t.Fatalf("Allow() unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("want request to be allowed")
	}
	if result.Remaining != 4 {
		t.Errorf("Remaining = %d, want 4", result.Remaining)
	}
}

func TestAllow_RequestBlocked(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "TokenBucket", nil
		},
		execFn: func(_ context.Context, _ string, _ algorithm.RateLimitAlgorithm, _ any) (storage.RateLimitResult, error) {
			return storage.RateLimitResult{Allowed: false, Remaining: 0}, nil
		},
	}

	engine := newTestEngine(t, rdb)
	result, err := engine.Allow(context.Background(), "ip:192.168.1.1")
	if err != nil {
		t.Fatalf("Allow() unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("want request to be blocked")
	}
}

func TestAllow_GetAlgorithmFailure(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("redis connection refused")
		},
	}

	engine := newTestEngine(t, rdb)
	_, err := engine.Allow(context.Background(), "ip:192.168.1.1")

	if err == nil {
		t.Fatal("want error when Redis unavailable, got nil")
	}
}

func TestAllow_InvalidAlgorithmStoredInRedis(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "CorruptedValue", nil
		},
	}

	engine := newTestEngine(t, rdb)
	_, err := engine.Allow(context.Background(), "ip:192.168.1.1")

	if err == nil {
		t.Fatal("want error for corrupted algorithm value, got nil")
	}
}

func TestAllow_ScriptExecutionError(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "TokenBucket", nil
		},
		execFn: func(_ context.Context, _ string, _ algorithm.RateLimitAlgorithm, _ any) (storage.RateLimitResult, error) {
			return storage.RateLimitResult{}, errors.New("lua script error")
		},
	}

	engine := newTestEngine(t, rdb)
	_, err := engine.Allow(context.Background(), "ip:192.168.1.1")

	if err == nil {
		t.Fatal("want error from script failure, got nil")
	}
}

func TestAllow_APIKeyClientType(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "TokenBucket", nil
		},
		execFn: func(_ context.Context, _ string, _ algorithm.RateLimitAlgorithm, _ any) (storage.RateLimitResult, error) {
			return storage.RateLimitResult{Allowed: true}, nil
		},
	}

	engine := newTestEngine(t, rdb)
	_, err := engine.Allow(context.Background(), "apikey:my-secret-key")
	if err != nil {
		t.Fatalf("Allow() unexpected error for apikey: prefix: %v", err)
	}
}

func TestGetCurrentAlgorithm_Success(t *testing.T) {
	tests := []struct {
		name     string
		redisVal string
		wantAlgo string
	}{
		{"token bucket", "TokenBucket", "TokenBucket"},
		{"sliding window log", "SlidingWindowLog", "SlidingWindowLog"},
		{"fixed window", "FixedWindow", "FixedWindow"},
		{"sliding window counter", "SlidingWindowCounter", "SlidingWindowCounter"},
		{"leaky bucket", "LeakyBucket", "LeakyBucket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdb := &mockRedisStorage{
				getFn: func(_ context.Context, _ string) (string, error) {
					return tt.redisVal, nil
				},
			}
			engine := newTestEngine(t, rdb)
			algo, err := engine.GetCurrentAlgorithm(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if algo != tt.wantAlgo {
				t.Errorf("algo = %q, want %q", algo, tt.wantAlgo)
			}
		})
	}
}

func TestGetCurrentAlgorithm_RedisError(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("connection timeout")
		},
	}

	engine := newTestEngine(t, rdb)
	_, err := engine.GetCurrentAlgorithm(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateAlgorithm_Success(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "TokenBucket", nil
		},
	}

	engine := newTestEngine(t, rdb)
	err := engine.UpdateAlgorithm(context.Background(), algorithm.AlgorithmSlidingWindowLog)
	if err != nil {
		t.Fatalf("UpdateAlgorithm() unexpected error: %v", err)
	}

	found := false
	for _, c := range rdb.setCalls {
		if c.key == algorithmConfigKey && c.value == "SlidingWindowLog" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Set(%q, %q), got calls: %+v", algorithmConfigKey, "SlidingWindowLog", rdb.setCalls)
	}
}

func TestUpdateAlgorithm_InvalidAlgorithm(t *testing.T) {
	engine := newTestEngine(t, &mockRedisStorage{})
	err := engine.UpdateAlgorithm(context.Background(), algorithm.RateLimitAlgorithm("NotValid"))

	if err == nil {
		t.Fatal("expected error for invalid algorithm, got nil")
	}
}

func TestUpdateAlgorithm_RedisWriteFailure(t *testing.T) {
	rdb := &mockRedisStorage{
		getFn: func(_ context.Context, _ string) (string, error) {
			return "TokenBucket", nil
		},
		setFn: func(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
			return false, errors.New("redis write failed")
		},
	}

	engine := newTestEngine(t, rdb)
	err := engine.UpdateAlgorithm(context.Background(), algorithm.AlgorithmFixedWindow)

	if err == nil {
		t.Fatal("expected error when Redis Set fails, got nil")
	}
}

func TestUpdateAlgorithm_AllValidAlgorithms(t *testing.T) {
	validAlgos := []algorithm.RateLimitAlgorithm{
		algorithm.AlgorithmTokenBucket,
		algorithm.AlgorithmLeakyBucket,
		algorithm.AlgorithmFixedWindow,
		algorithm.AlgorithmSlidingWindowLog,
		algorithm.AlgorithmSlidingWindowCounter,
	}

	for _, algo := range validAlgos {
		t.Run(algo.String(), func(t *testing.T) {
			rdb := &mockRedisStorage{
				getFn: func(_ context.Context, _ string) (string, error) {
					return "TokenBucket", nil
				},
			}
			engine := newTestEngine(t, rdb)
			err := engine.UpdateAlgorithm(context.Background(), algo)
			if err != nil {
				t.Errorf("UpdateAlgorithm(%q) unexpected error: %v", algo, err)
			}
		})
	}
}

func TestGetFailOpen(t *testing.T) {
	tests := []struct {
		name     string
		redisVal string
		redisErr error
		want     bool
		wantErr  bool
	}{
		{"true value", "true", nil, true, false},
		{"false value", "false", nil, false, false},
		{"redis error", "", errors.New("unavailable"), false, true},
		{"invalid value", "not_a_bool", nil, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdb := &mockRedisStorage{
				getFn: func(_ context.Context, _ string) (string, error) {
					return tt.redisVal, tt.redisErr
				},
			}
			engine := newTestEngine(t, rdb)
			got, err := engine.GetFailOpen(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("GetFailOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetFailOpen_UpdatesInMemoryConfig(t *testing.T) {
	engine := newTestEngine(t, &mockRedisStorage{})
	engine.rateLimitConfig.FailOpen = false

	_, err := engine.SetFailOpen(context.Background(), true)
	if err != nil {
		t.Fatalf("SetFailOpen() unexpected error: %v", err)
	}
	if !engine.rateLimitConfig.FailOpen {
		t.Error("expected in-memory FailOpen to be updated to true")
	}
}

func TestSetFailOpen_WritesToRedis(t *testing.T) {
	rdb := &mockRedisStorage{}
	engine := newTestEngine(t, rdb)

	_, err := engine.SetFailOpen(context.Background(), true)
	if err != nil {
		t.Fatalf("SetFailOpen() unexpected error: %v", err)
	}

	found := false
	for _, c := range rdb.setCalls {
		if c.key == failOpenConfigKey && c.value == "true" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Set(%q, %q), got calls: %+v", failOpenConfigKey, "true", rdb.setCalls)
	}
}

func TestSetFailOpen_RedisWriteFailure(t *testing.T) {
	rdb := &mockRedisStorage{
		setFn: func(_ context.Context, _ string, _ any, _ time.Duration) (bool, error) {
			return false, errors.New("redis write failed")
		},
	}

	engine := newTestEngine(t, rdb)
	_, err := engine.SetFailOpen(context.Background(), true)

	if err == nil {
		t.Fatal("expected error when Redis write fails, got nil")
	}
}

func TestListAlgorithm_ReturnsNonEmpty(t *testing.T) {
	engine := newTestEngine(t, &mockRedisStorage{})
	algos := engine.ListAlgorithm()

	if len(algos) == 0 {
		t.Error("expected at least one algorithm, got none")
	}
}

func TestListAlgorithm_ContainsExpectedAlgorithms(t *testing.T) {
	engine := newTestEngine(t, &mockRedisStorage{})
	algos := engine.ListAlgorithm()

	expected := []string{
		"TokenBucket",
		"LeakyBucket",
		"FixedWindow",
		"SlidingWindowLog",
		"SlidingWindowCounter",
	}

	algoSet := make(map[string]bool, len(algos))
	for _, a := range algos {
		algoSet[a] = true
	}

	for _, want := range expected {
		if !algoSet[want] {
			t.Errorf("ListAlgorithm() missing %q", want)
		}
	}
}

func TestGetClientTypeFromKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"apikey prefix", "apikey:abc123", "apikey"},
		{"apikey prefix long", "apikey:some-long-key-value", "apikey"},
		{"ip address", "ip:192.168.1.1", "ip"},
		{"empty string", "", "ip"},
		{"too short for prefix", "apikey", "ip"},
		{"seven chars no prefix", "1234567", "ip"},
		{"similar but wrong prefix", "APIKEY:x", "ip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getClientTypeFromKey(tt.key)
			if got != tt.want {
				t.Errorf("getClientTypeFromKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
