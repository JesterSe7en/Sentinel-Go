package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
)

var (
	ErrMissingRedisMasterName = errors.New("config: REDIS_MASTERNAME is required")
	ErrMissingRedisSentinels  = errors.New("config: REDIS_SENTINELS is required")
	ErrInvalidRedisDB         = errors.New("config: REDIS_DB must be a valid integer")
	ErrMissingRateLimitAlgo   = errors.New("config: RATE_LIMIT_ALGORITHM is required")
	ErrInvalidShutdownTimeout = errors.New("config: SHUTDOWN_TIMEOUT must be a valid integer")
	ErrMissingHTTPPort        = errors.New("config: HTTP_PORT is required")
	ErrMissingGRPCPort        = errors.New("config: GRPC_PORT is required")
	ErrMissingLogLevel        = errors.New("config: LOG_LEVEL is required")
	ErrMissingLogPath         = errors.New("config: LOG_PATH is required")
	ErrInvalidLogPath         = errors.New("config: LOG_PATH must be a valid path")
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
	ShutdownTimeout time.Duration
}

type RateLimitConfig struct {
	Algorithm            string
	FailOpen             bool
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
		return nil, fmt.Errorf("invalid algorithm: %w", err)
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

	rateLimitConfig, err := loadRateLimitConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load rate limit config: %w", err)
	}

	serverConfig, err := loadServerConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load server config: %w", err)
	}

	return &SentinelAppConfig{
		BootstrapCfg: bootstrapConfig,
		RedisCfg:     redisConfig,
		RateLimitCfg: rateLimitConfig,
		ServerCfg:    serverConfig,
	}, nil
}

func loadRedisConfig() (RedisConfig, error) {
	name := os.Getenv("REDIS_MASTERNAME")
	if name == "" {
		return RedisConfig{}, ErrMissingRedisMasterName
	}

	rawSentinels := os.Getenv("REDIS_SENTINELS")
	if rawSentinels == "" {
		return RedisConfig{}, ErrMissingRedisSentinels
	}

	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		return RedisConfig{}, fmt.Errorf("%w: %w", ErrInvalidRedisDB, err)
	}

	return RedisConfig{
		MasterName:    name,
		SentinelAddrs: strings.Split(rawSentinels, ","),
		DB:            redisDB,
		Password:      os.Getenv("REDIS_PASSWORD"),
	}, nil
}

func loadBootstrapConfig() (BootstrapConfig, error) {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		return BootstrapConfig{}, ErrMissingLogLevel
	}

	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		return BootstrapConfig{}, ErrMissingLogPath
	}

	// check if logPath is a valid path
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return BootstrapConfig{}, ErrInvalidLogPath
	}

	timeout, err := strconv.Atoi(os.Getenv("SHUTDOWN_TIMEOUT"))
	if err != nil {
		return BootstrapConfig{}, fmt.Errorf("%w: %w", ErrInvalidShutdownTimeout, err)
	}

	return BootstrapConfig{
		LogLevel:        logLevel,
		LogPath:         logPath,
		ShutdownTimeout: time.Duration(timeout) * time.Second,
	}, nil
}

func loadRateLimitConfig() (RateLimitConfig, error) {
	algo := os.Getenv("RATE_LIMIT_ALGORITHM")
	if algo == "" {
		return RateLimitConfig{}, ErrMissingRateLimitAlgo
	}

	return RateLimitConfig{
		Algorithm: algo,
		FailOpen:  os.Getenv("RATE_LIMIT_FAIL_OPEN") == "true",
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
	}, nil
}

func loadServerConfig() (ServerConfig, error) {
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		return ServerConfig{}, ErrMissingHTTPPort
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		return ServerConfig{}, ErrMissingGRPCPort
	}

	return ServerConfig{
		HTTPPort: httpPort,
		GRPCPort: grpcPort,
	}, nil
}
