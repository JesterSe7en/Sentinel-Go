package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
)

type SentinelAppConfig struct {
	BootstrapCfg BootstrapConfig
	RedisCfg     RedisConfig
	RateLimitCfg RateLimitConfig
	ServerCfg    ServerConfig
}

type BootstrapConfig struct {
	LogLevel        string
	LogPath         string
	LogFormat       string // TODO: maybe not needed?
	ShutdownTimeout time.Duration
}

type RateLimitConfig struct {
	Algorithm            string
	TokenBucket          *TokenBucketConfig
	LeakyBucket          *LeakyBucketConfig
	FixedWindow          *FixedWindowConfig
	SlidingWindowLog     *SlidingWindowLogConfig
	SlidingWindowCounter *SlidingWindowCounterConfig
}

type TokenBucketConfig struct {
	RefillRate float64
	Capacity   int
}

type LeakyBucketConfig struct {
	Capacity int
	LeakRate float64
}

type FixedWindowConfig struct {
	Limit  int
	Window time.Duration
}

type SlidingWindowLogConfig struct {
	Limit             int
	SlidingWindowSize int
}

type SlidingWindowCounterConfig struct {
	Limit             int
	SlidingWindowSize int
	BucketSize        int
}

func (c *RateLimitConfig) GetConfigForAlgorithm(algo string) (any, error) {
	parsed, err := algorithm.ParseAlgorithm(algo)
	if err != nil {
		return nil, fmt.Errorf("invalid algorithm: %v", err)
	}
	switch parsed {
	case algorithm.AlgorithmTokenBucket:
		return c.TokenBucket, nil
	case algorithm.AlgorithmLeakyBucket:
		return c.LeakyBucket, nil
	case algorithm.AlgorithmFixedWindow:
		return c.FixedWindow, nil
	case algorithm.AlgorithmSlidingWindowLog:
		return c.SlidingWindowLog, nil
	case algorithm.AlgorithmSlidingWindowCounter:
		return c.SlidingWindowCounter, nil
	default:
		return nil, fmt.Errorf("unknown algorithm: %s", algo)
	}
}

type RedisConfig struct {
	MasterName    string
	SentinelAddrs []string
	Password      string
	DB            int
}

type ServerConfig struct {
	HTTPPort string
	GRPCPort string
}

func Load() (*SentinelAppConfig, error) {
	redisConfig, err := loadRedisConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load redis config: %w", err)
	}

	bootstrapConfig, err := loadBootstrapConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load bootstrap config: %w", err)
	}

	cfg := &SentinelAppConfig{
		BootstrapCfg: bootstrapConfig,
		RedisCfg:     redisConfig,
		RateLimitCfg: defaultRateLimitConfig(),
		ServerCfg:    loadSeverConfig(),
	}

	return cfg, nil
}

func defaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Algorithm: os.Getenv("RATE_LIMIT_ALGORITHM"),
		TokenBucket: &TokenBucketConfig{
			RefillRate: 1,
			Capacity:   5,
		},

		LeakyBucket: &LeakyBucketConfig{
			Capacity: 100,
			LeakRate: 10.0,
		},
		FixedWindow: &FixedWindowConfig{
			Limit:  100,
			Window: time.Minute,
		},
		SlidingWindowLog: &SlidingWindowLogConfig{
			Limit:             100,
			SlidingWindowSize: 60,
		},
		SlidingWindowCounter: &SlidingWindowCounterConfig{
			Limit:             100,
			SlidingWindowSize: 60,
			BucketSize:        10,
		},
	}
}

func loadBootstrapConfig() (BootstrapConfig, error) {
	timeout, err := strconv.Atoi(os.Getenv("SHUTDOWN_TIMEOUT"))
	if err != nil {
		return BootstrapConfig{}, fmt.Errorf("invalid shutdown timeout: %w", err)
	}

	return BootstrapConfig{
		LogLevel:        os.Getenv("LOG_LEVEL"),
		LogPath:         os.Getenv("LOG_PATH"),
		LogFormat:       "hehe",
		ShutdownTimeout: time.Duration(timeout) * time.Second,
	}, nil
}

func loadRedisConfig() (RedisConfig, error) {
	name := os.Getenv("REDIS_MASTERNAME")
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		return RedisConfig{}, fmt.Errorf("invalid redis db: %w", err)
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")

	rawSentinels := os.Getenv("REDIS_SENTINELS")
	sentinelSlice := strings.Split(rawSentinels, ",")
	if rawSentinels == "" {
		return RedisConfig{}, fmt.Errorf("REDIS_SENTINELS environment variable is not set")
	}

	return RedisConfig{
		MasterName:    name,
		SentinelAddrs: sentinelSlice,
		DB:            redisDB,
		Password:      redisPassword,
	}, nil
}

func loadSeverConfig() ServerConfig {
	return ServerConfig{
		HTTPPort: os.Getenv("HTTP_PORT"),
		GRPCPort: os.Getenv("GRPC_PORT"),
	}
}
