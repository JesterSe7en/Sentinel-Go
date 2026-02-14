package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
}

func TestLoad_MissingEnvVars(t *testing.T) {
	requiredVars := []string{
		"REDIS_MASTERNAME",
		"REDIS_SENTINELS",
		"REDIS_DB",
		"REDIS_PASSWORD",
		"RATE_LIMIT_ALGORITHM",
		"HTTP_PORT",
		"GRPC_PORT",
		"LOG_LEVEL",
		"LOG_PATH",
		"SHUTDOWN_TIMEOUT",
	}

	// Save and unset all required vars
	origEnv := make(map[string]string)
	for _, v := range requiredVars {
		origEnv[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range origEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	_, err := Load()
	if err == nil {
		t.Error("expected error when missing env vars, got nil")
	}
}

func TestLoad_InvalidRedisDB(t *testing.T) {
	orig := os.Getenv("REDIS_DB")
	defer os.Setenv("REDIS_DB", orig)

	setEnv(t, "REDIS_DB", "not_a_number")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid REDIS_DB, got nil")
	}
}

func TestLoad_InvalidShutdownTimeout(t *testing.T) {
	orig := os.Getenv("SHUTDOWN_TIMEOUT")
	defer os.Setenv("SHUTDOWN_TIMEOUT", orig)

	setEnv(t, "SHUTDOWN_TIMEOUT", "not_a_number")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid SHUTDOWN_TIMEOUT, got nil")
	}
}

func TestLoad_MissingSentinels(t *testing.T) {
	orig := os.Getenv("REDIS_SENTINELS")
	defer os.Setenv("REDIS_SENTINELS", orig)

	os.Unsetenv("REDIS_SENTINELS")

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing REDIS_SENTINELS, got nil")
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := defaultRateLimitConfig()

	if cfg.TokenBucket == nil {
		t.Error("TokenBucket config should not be nil")
	}
	if cfg.LeakyBucket == nil {
		t.Error("LeakyBucket config should not be nil")
	}
	if cfg.FixedWindow == nil {
		t.Error("FixedWindow config should not be nil")
	}
	if cfg.SlidingWindowLog == nil {
		t.Error("SlidingWindowLog config should not be nil")
	}
	if cfg.SlidingWindowCounter == nil {
		t.Error("SlidingWindowCounter config should not be nil")
	}

	if cfg.TokenBucket.Capacity != 5 {
		t.Errorf("TokenBucket.Capacity = %d, want 5", cfg.TokenBucket.Capacity)
	}
	if cfg.TokenBucket.RefillRate != 1 {
		t.Errorf("TokenBucket.RefillRate = %v, want 1", cfg.TokenBucket.RefillRate)
	}
	if cfg.LeakyBucket.Capacity != 100 {
		t.Errorf("LeakyBucket.Capacity = %d, want 100", cfg.LeakyBucket.Capacity)
	}
	if cfg.LeakyBucket.LeakRate != 10.0 {
		t.Errorf("LeakyBucket.LeakRate = %v, want 10.0", cfg.LeakyBucket.LeakRate)
	}
	if cfg.FixedWindow.Limit != 100 {
		t.Errorf("FixedWindow.Limit = %d, want 100", cfg.FixedWindow.Limit)
	}
	if cfg.FixedWindow.Window != time.Minute {
		t.Errorf("FixedWindow.Window = %v, want 1m", cfg.FixedWindow.Window)
	}
}

func TestRateLimitConfig_GetConfigForAlgorithm(t *testing.T) {
	cfg := defaultRateLimitConfig()

	tests := []struct {
		name        string
		algo        string
		expectError bool
		checkFn     func(any) bool
	}{
		{
			name:        "TokenBucket",
			algo:        "TokenBucket",
			expectError: false,
			checkFn: func(v any) bool {
				c, ok := v.(*TokenBucketConfig)
				return ok && c.Capacity == 5 && c.RefillRate == 1
			},
		},
		{
			name:        "LeakyBucket",
			algo:        "LeakyBucket",
			expectError: false,
			checkFn: func(v any) bool {
				c, ok := v.(*LeakyBucketConfig)
				return ok && c.Capacity == 100 && c.LeakRate == 10.0
			},
		},
		{
			name:        "FixedWindow",
			algo:        "FixedWindow",
			expectError: false,
			checkFn: func(v any) bool {
				c, ok := v.(*FixedWindowConfig)
				return ok && c.Limit == 100 && c.Window == time.Minute
			},
		},
		{
			name:        "SlidingWindowLog",
			algo:        "SlidingWindowLog",
			expectError: false,
			checkFn: func(v any) bool {
				c, ok := v.(*SlidingWindowLogConfig)
				return ok && c.Limit == 100 && c.SlidingWindowSize == 60
			},
		},
		{
			name:        "SlidingWindowCounter",
			algo:        "SlidingWindowCounter",
			expectError: false,
			checkFn: func(v any) bool {
				c, ok := v.(*SlidingWindowCounterConfig)
				return ok && c.Limit == 100 && c.SlidingWindowSize == 60 && c.BucketSize == 10
			},
		},
		{
			name:        "InvalidAlgorithm",
			algo:        "InvalidAlgo",
			expectError: true,
			checkFn:     nil,
		},
		{
			name:        "EmptyString",
			algo:        "",
			expectError: true,
			checkFn:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cfg.GetConfigForAlgorithm(tt.algo)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for algo %q, got nil", tt.algo)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for algo %q: %v", tt.algo, err)
				return
			}

			if !tt.checkFn(result) {
				t.Errorf("unexpected result for algo %q: %+v", tt.algo, result)
			}
		})
	}
}

func TestLoadRedisConfig_Valid(t *testing.T) {
	orig := make(map[string]string)
	vars := []string{"REDIS_MASTERNAME", "REDIS_SENTINELS", "REDIS_DB", "REDIS_PASSWORD"}
	for _, v := range vars {
		orig[v] = os.Getenv(v)
		defer os.Setenv(v, orig[v])
	}

	setEnv(t, "REDIS_MASTERNAME", "redis-master")
	setEnv(t, "REDIS_SENTINELS", "localhost:26379,localhost:26380")
	setEnv(t, "REDIS_DB", "0")
	setEnv(t, "REDIS_PASSWORD", "password123")

	cfg, err := loadRedisConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MasterName != "redis-master" {
		t.Errorf("MasterName = %q, want 'redis-master'", cfg.MasterName)
	}
	if len(cfg.SentinelAddrs) != 2 {
		t.Errorf("SentinelAddrs length = %d, want 2", len(cfg.SentinelAddrs))
	}
	if cfg.DB != 0 {
		t.Errorf("DB = %d, want 0", cfg.DB)
	}
	if cfg.Password != "password123" {
		t.Errorf("Password = %q, want 'password123'", cfg.Password)
	}
}

func TestLoadRedisConfig_MissingSentinels(t *testing.T) {
	orig := os.Getenv("REDIS_SENTINELS")
	defer os.Setenv("REDIS_SENTINELS", orig)

	os.Unsetenv("REDIS_SENTINELS")

	_, err := loadRedisConfig()
	if err == nil {
		t.Error("expected error for missing REDIS_SENTINELS, got nil")
	}
}

func TestLoadServerConfig(t *testing.T) {
	origHTTP := os.Getenv("HTTP_PORT")
	origGRPC := os.Getenv("GRPC_PORT")
	defer func() {
		os.Setenv("HTTP_PORT", origHTTP)
		os.Setenv("GRPC_PORT", origGRPC)
	}()

	setEnv(t, "HTTP_PORT", "9090")
	setEnv(t, "GRPC_PORT", "50052")

	cfg := loadSeverConfig()

	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort = %q, want '9090'", cfg.HTTPPort)
	}
	if cfg.GRPCPort != "50052" {
		t.Errorf("GRPCPort = %q, want '50052'", cfg.GRPCPort)
	}
}

func TestLoadBootstrapConfig(t *testing.T) {
	orig := make(map[string]string)
	vars := []string{"SHUTDOWN_TIMEOUT", "LOG_LEVEL", "LOG_PATH"}
	for _, v := range vars {
		orig[v] = os.Getenv(v)
		defer os.Setenv(v, orig[v])
	}

	setEnv(t, "SHUTDOWN_TIMEOUT", "30")
	setEnv(t, "LOG_LEVEL", "debug")
	setEnv(t, "LOG_PATH", "/var/log/app.log")

	cfg, err := loadBootstrapConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want 'debug'", cfg.LogLevel)
	}
	if cfg.LogPath != "/var/log/app.log" {
		t.Errorf("LogPath = %q, want '/var/log/app.log'", cfg.LogPath)
	}
}
