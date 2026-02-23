package config

import (
	"errors"
	"os"
	"testing"
	"time"
)

var validEnv = map[string]string{
	"REDIS_MASTERNAME":     "mymaster",
	"REDIS_SENTINELS":      "localhost:26379",
	"REDIS_DB":             "0",
	"REDIS_PASSWORD":       "supersecret",
	"RATE_LIMIT_ALGORITHM": "TokenBucket",
	"HTTP_PORT":            "8080",
	"GRPC_PORT":            "9090",
	"LOG_LEVEL":            "info",
	"LOG_PATH":             "/tmp/sentinel.log",
	"SHUTDOWN_TIMEOUT":     "30",
}

func setValidEnv(t *testing.T) {
	t.Helper()
	for k, v := range validEnv {
		t.Setenv(k, v)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	setValidEnv(t)
	t.Setenv("REDIS_SENTINELS", "localhost:26379,localhost:26380")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.RedisCfg.MasterName != "mymaster" {
		t.Errorf("MasterName = %q, want %q", cfg.RedisCfg.MasterName, "mymaster")
	}
	if len(cfg.RedisCfg.SentinelAddrs) != 2 {
		t.Errorf("SentinelAddrs len = %d, want 2", len(cfg.RedisCfg.SentinelAddrs))
	}
	if cfg.RedisCfg.SentinelAddrs[0] != "localhost:26379" {
		t.Errorf("SentinelAddrs[0] = %q, want %q", cfg.RedisCfg.SentinelAddrs[0], "localhost:26379")
	}
	if cfg.RedisCfg.DB != 0 {
		t.Errorf("DB = %d, want 0", cfg.RedisCfg.DB)
	}
	if cfg.ServerCfg.HTTPPort != "8080" {
		t.Errorf("HTTPPort = %q, want %q", cfg.ServerCfg.HTTPPort, "8080")
	}
	if cfg.ServerCfg.GRPCPort != "9090" {
		t.Errorf("GRPCPort = %q, want %q", cfg.ServerCfg.GRPCPort, "9090")
	}
	if cfg.BootstrapCfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.BootstrapCfg.LogLevel, "info")
	}
	if cfg.BootstrapCfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.BootstrapCfg.ShutdownTimeout)
	}
	if cfg.RateLimitCfg.Algorithm != "TokenBucket" {
		t.Errorf("Algorithm = %q, want %q", cfg.RateLimitCfg.Algorithm, "TokenBucket")
	}
}

func TestLoad_MissingEnvVars(t *testing.T) {
	tests := []struct {
		name      string
		unsetVar  string
		wantErrIs error
	}{
		{"missing REDIS_MASTERNAME", "REDIS_MASTERNAME", ErrMissingRedisMasterName},
		{"missing REDIS_SENTINELS", "REDIS_SENTINELS", ErrMissingRedisSentinels},
		{"missing REDIS_DB", "REDIS_DB", ErrInvalidRedisDB},
		{"missing RATE_LIMIT_ALGORITHM", "RATE_LIMIT_ALGORITHM", ErrMissingRateLimitAlgo},
		{"missing HTTP_PORT", "HTTP_PORT", ErrMissingHTTPPort},
		{"missing GRPC_PORT", "GRPC_PORT", ErrMissingGRPCPort},
		{"missing LOG_LEVEL", "LOG_LEVEL", ErrMissingLogLevel},
		{"missing LOG_PATH", "LOG_PATH", ErrMissingLogPath},
		{"missing SHUTDOWN_TIMEOUT", "SHUTDOWN_TIMEOUT", ErrInvalidShutdownTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			os.Unsetenv(tt.unsetVar)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() expected error when %q is unset, got nil", tt.unsetVar)
			}
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Load() error = %v, want errors.Is(%v)", err, tt.wantErrIs)
			}
		})
	}
}

func TestLoad_InvalidFormats(t *testing.T) {
	tests := []struct {
		name      string
		envKey    string
		envVal    string
		wantErrIs error
	}{
		{"REDIS_DB not a number", "REDIS_DB", "not_a_number", ErrInvalidRedisDB},
		{"REDIS_DB is float", "REDIS_DB", "1.5", ErrInvalidRedisDB},
		{"SHUTDOWN_TIMEOUT not a number", "SHUTDOWN_TIMEOUT", "not_a_number", ErrInvalidShutdownTimeout},
		{"SHUTDOWN_TIMEOUT is float", "SHUTDOWN_TIMEOUT", "30.5", ErrInvalidShutdownTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.envKey, tt.envVal)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() expected error for %s=%q, got nil", tt.envKey, tt.envVal)
			}
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Load() error = %v, want errors.Is(%v)", err, tt.wantErrIs)
			}
		})
	}
}

func TestLoad_OptionalRedisPassword(t *testing.T) {
	setValidEnv(t)
	os.Unsetenv("REDIS_PASSWORD")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should succeed without REDIS_PASSWORD, got: %v", err)
	}
	if cfg.RedisCfg.Password != "" {
		t.Errorf("Password = %q, want empty string", cfg.RedisCfg.Password)
	}
}

func TestLoad_FailOpen(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		wantBool bool
	}{
		{"true enables fail open", "true", true},
		{"false disables fail open", "false", false},
		{"empty defaults to false", "", false},
		{"arbitrary string defaults to false", "yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv("RATE_LIMIT_FAIL_OPEN", tt.envVal)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if cfg.RateLimitCfg.FailOpen != tt.wantBool {
				t.Errorf("FailOpen = %v, want %v", cfg.RateLimitCfg.FailOpen, tt.wantBool)
			}
		})
	}
}

func TestLoadRedisConfig_Valid(t *testing.T) {
	t.Setenv("REDIS_MASTERNAME", "redis-master")
	t.Setenv("REDIS_SENTINELS", "localhost:26379,localhost:26380")
	t.Setenv("REDIS_DB", "0")
	t.Setenv("REDIS_PASSWORD", "password123")

	cfg, err := loadRedisConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MasterName != "redis-master" {
		t.Errorf("MasterName = %q, want %q", cfg.MasterName, "redis-master")
	}
	if len(cfg.SentinelAddrs) != 2 {
		t.Errorf("SentinelAddrs len = %d, want 2", len(cfg.SentinelAddrs))
	}
	if cfg.SentinelAddrs[0] != "localhost:26379" {
		t.Errorf("SentinelAddrs[0] = %q, want %q", cfg.SentinelAddrs[0], "localhost:26379")
	}
	if cfg.DB != 0 {
		t.Errorf("DB = %d, want 0", cfg.DB)
	}
	if cfg.Password != "password123" {
		t.Errorf("Password = %q, want %q", cfg.Password, "password123")
	}
}

func TestLoadRedisConfig_SingleSentinel(t *testing.T) {
	t.Setenv("REDIS_MASTERNAME", "mymaster")
	t.Setenv("REDIS_SENTINELS", "localhost:26379")
	t.Setenv("REDIS_DB", "0")

	cfg, err := loadRedisConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.SentinelAddrs) != 1 {
		t.Errorf("SentinelAddrs len = %d, want 1", len(cfg.SentinelAddrs))
	}
	if cfg.SentinelAddrs[0] != "localhost:26379" {
		t.Errorf("SentinelAddrs[0] = %q, want %q", cfg.SentinelAddrs[0], "localhost:26379")
	}
}

func TestLoadRedisConfig_MissingFields(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		wantErrIs error
	}{
		{
			name: "missing master name",
			setup: func(t *testing.T) {
				t.Setenv("REDIS_SENTINELS", "localhost:26379")
				t.Setenv("REDIS_DB", "0")
				os.Unsetenv("REDIS_MASTERNAME")
			},
			wantErrIs: ErrMissingRedisMasterName,
		},
		{
			name: "missing sentinels",
			setup: func(t *testing.T) {
				t.Setenv("REDIS_MASTERNAME", "mymaster")
				t.Setenv("REDIS_DB", "0")
				os.Unsetenv("REDIS_SENTINELS")
			},
			wantErrIs: ErrMissingRedisSentinels,
		},
		{
			name: "invalid DB format",
			setup: func(t *testing.T) {
				t.Setenv("REDIS_MASTERNAME", "mymaster")
				t.Setenv("REDIS_SENTINELS", "localhost:26379")
				t.Setenv("REDIS_DB", "not_a_number")
			},
			wantErrIs: ErrInvalidRedisDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			_, err := loadRedisConfig()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("error = %v, want errors.Is(%v)", err, tt.wantErrIs)
			}
		})
	}
}

func TestLoadBootstrapConfig_Valid(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "30")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_PATH", "/var/log/app.log")

	cfg, err := loadBootstrapConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogPath != "/var/log/app.log" {
		t.Errorf("LogPath = %q, want %q", cfg.LogPath, "/var/log/app.log")
	}
}

func TestLoadBootstrapConfig_MissingInvalidFields(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		wantErrIs error
	}{
		{
			name: "missing LOG_LEVEL",
			setup: func(t *testing.T) {
				t.Setenv("LOG_PATH", "/var/log/app.log")
				t.Setenv("SHUTDOWN_TIMEOUT", "30")
				os.Unsetenv("LOG_LEVEL")
			},
			wantErrIs: ErrMissingLogLevel,
		},
		{
			name: "missing LOG_PATH",
			setup: func(t *testing.T) {
				t.Setenv("LOG_LEVEL", "info")
				t.Setenv("SHUTDOWN_TIMEOUT", "30")
				os.Unsetenv("LOG_PATH")
			},
			wantErrIs: ErrMissingLogPath,
		},
		{
			name: "invalid SHUTDOWN_TIMEOUT",
			setup: func(t *testing.T) {
				t.Setenv("LOG_LEVEL", "info")
				t.Setenv("LOG_PATH", "/var/log/app.log")
				t.Setenv("SHUTDOWN_TIMEOUT", "bad")
			},
			wantErrIs: ErrInvalidShutdownTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			_, err := loadBootstrapConfig()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("error = %v, want errors.Is(%v)", err, tt.wantErrIs)
			}
		})
	}
}

func TestLoadServerConfig_Valid(t *testing.T) {
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("GRPC_PORT", "50052")

	cfg, err := loadServerConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "9090")
	}
	if cfg.GRPCPort != "50052" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "50052")
	}
}

func TestLoadServerConfig_MissingFields(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		wantErrIs error
	}{
		{
			name: "missing HTTP_PORT",
			setup: func(t *testing.T) {
				t.Setenv("GRPC_PORT", "9090")
				os.Unsetenv("HTTP_PORT")
			},
			wantErrIs: ErrMissingHTTPPort,
		},
		{
			name: "missing GRPC_PORT",
			setup: func(t *testing.T) {
				t.Setenv("HTTP_PORT", "8080")
				os.Unsetenv("GRPC_PORT")
			},
			wantErrIs: ErrMissingGRPCPort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			_, err := loadServerConfig()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("error = %v, want errors.Is(%v)", err, tt.wantErrIs)
			}
		})
	}
}

func TestRateLimitConfig_GetConfigForAlgorithm(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		algo    string
		wantErr bool
		checkFn func(t *testing.T, v any)
	}{
		{
			name: "TokenBucket returns correct config",
			algo: "TokenBucket",
			checkFn: func(t *testing.T, v any) {
				t.Helper()
				c, ok := v.(*TokenBucketConfig)
				if !ok {
					t.Fatalf("expected *TokenBucketConfig, got %T", v)
				}
				if c.Capacity != 5 {
					t.Errorf("Capacity = %d, want 5", c.Capacity)
				}
				if c.RefillRate != 1 {
					t.Errorf("RefillRate = %v, want 1", c.RefillRate)
				}
			},
		},
		{
			name: "LeakyBucket returns correct config",
			algo: "LeakyBucket",
			checkFn: func(t *testing.T, v any) {
				t.Helper()
				c, ok := v.(*LeakyBucketConfig)
				if !ok {
					t.Fatalf("expected *LeakyBucketConfig, got %T", v)
				}
				if c.Capacity != 100 {
					t.Errorf("Capacity = %d, want 100", c.Capacity)
				}
				if c.LeakRate != 10.0 {
					t.Errorf("LeakRate = %v, want 10.0", c.LeakRate)
				}
			},
		},
		{
			name: "FixedWindow returns correct config",
			algo: "FixedWindow",
			checkFn: func(t *testing.T, v any) {
				t.Helper()
				c, ok := v.(*FixedWindowConfig)
				if !ok {
					t.Fatalf("expected *FixedWindowConfig, got %T", v)
				}
				if c.Limit != 100 {
					t.Errorf("Limit = %d, want 100", c.Limit)
				}
				if c.Window != time.Minute {
					t.Errorf("Window = %v, want 1m", c.Window)
				}
			},
		},
		{
			name: "SlidingWindowLog returns correct config",
			algo: "SlidingWindowLog",
			checkFn: func(t *testing.T, v any) {
				t.Helper()
				c, ok := v.(*SlidingWindowLogConfig)
				if !ok {
					t.Fatalf("expected *SlidingWindowLogConfig, got %T", v)
				}
				if c.Limit != 100 {
					t.Errorf("Limit = %d, want 100", c.Limit)
				}
				if c.SlidingWindowSize != 60 {
					t.Errorf("SlidingWindowSize = %d, want 60", c.SlidingWindowSize)
				}
			},
		},
		{
			name: "SlidingWindowCounter returns correct config",
			algo: "SlidingWindowCounter",
			checkFn: func(t *testing.T, v any) {
				t.Helper()
				c, ok := v.(*SlidingWindowCounterConfig)
				if !ok {
					t.Fatalf("expected *SlidingWindowCounterConfig, got %T", v)
				}
				if c.Limit != 100 {
					t.Errorf("Limit = %d, want 100", c.Limit)
				}
				if c.BucketSize != 10 {
					t.Errorf("BucketSize = %d, want 10", c.BucketSize)
				}
			},
		},
		{
			name:    "invalid algorithm returns error",
			algo:    "InvalidAlgo",
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			algo:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cfg.RateLimitCfg.GetConfigForAlgorithm(tt.algo)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for algo %q, got nil", tt.algo)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for algo %q: %v", tt.algo, err)
			}
			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}
