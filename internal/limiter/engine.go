package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"github.com/redis/go-redis/v9"
)

type SentinelEngine struct {
	client *redis.Client
	log    *logger.Logger

	rateLimitConfig *config.RateLimitConfig
}

const algorithmConfigKey = "sentinel:global:algorithm"

func NewSentinelEngine(client *redis.Client, log *logger.Logger, cfg *config.SentinelAppConfig) (*SentinelEngine, error) {
	initialAlgo, err := algorithm.ParseAlgorithm(cfg.RateLimitCfg.Algorithm)

	if err != nil {
		return nil, fmt.Errorf("invalid initial algorithm specified in config: %w", err)
	}
	if _, exists := storage.Scripts[initialAlgo.String()]; !exists {
		return nil, fmt.Errorf("unknown limit algo specified: %s", initialAlgo)
	}

	engine := &SentinelEngine{
		client:          client,
		log:             log,
		rateLimitConfig: &cfg.RateLimitCfg,
	}

	err = client.SetNX(context.Background(), algorithmConfigKey, initialAlgo.String(), 0).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set default algorithm: %w", err)
	}

	return engine, nil
}

func (se *SentinelEngine) Allow(ctx context.Context, key string) (bool, error) {
	algo, err := se.GetCurrentAlgorithm(ctx)
	if err != nil {
		return false, err
	}

	changeTo, err := algorithm.ParseAlgorithm(algo)
	if err != nil {
		return false, err
	}

	success, err := se.checkAllow(ctx, key, changeTo)

	return success, err
}

func (se *SentinelEngine) ListAlgorithm() []string {
	return storage.GetAlgorithmNames()
}

func (se *SentinelEngine) GetCurrentAlgorithm(ctx context.Context) (string, error) {
	algo, err := se.client.Get(ctx, algorithmConfigKey).Result()
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

	// Update in Redis - ALL instances will see this
	err := se.client.Set(ctx, algorithmConfigKey, algo.String(), 0).Err()
	if err != nil {
		return fmt.Errorf("failed to update algorithm: %w", err)
	}

	return nil
}

func (se *SentinelEngine) checkAllow(ctx context.Context, key string, algo algorithm.RateLimitAlgorithm) (bool, error) {
	cfg, err := se.rateLimitConfig.GetConfigForAlgorithm(algo.String())
	if err != nil {
		return false, fmt.Errorf("failed to get config for algorithm: %w", err)
	}

	args := se.buildArgsFromConfig(algo, cfg)
	luaScriptToRun := storage.Scripts[algo.String()]
	result, err := luaScriptToRun.Run(ctx, se.client, []string{key}, args).Int()
	if err != nil {
		return false, fmt.Errorf("error running script: %w", err)
	}

	// result == 1 means allowed; result == 0 means not allowed
	return result == 1, nil
}

func (se *SentinelEngine) buildArgsFromConfig(algo algorithm.RateLimitAlgorithm, cfg any) []interface{} {
	if cfg == nil {
		return nil
	}

	now := time.Now().Unix()

	switch c := cfg.(type) {
	case *config.TokenBucketConfig:
		return []interface{}{c.Capacity, c.RefillRate, now}
	case *config.LeakyBucketConfig:
		return []interface{}{c.Capacity, c.LeakRate, now}
	case *config.FixedWindowConfig:
		windowSecs := int(c.Window.Seconds())
		return []interface{}{c.Limit, windowSecs}
	case *config.SlidingWindowLogConfig:
		return []interface{}{c.Limit, c.SlidingWindowSize, now}
	case *config.SlidingWindowCounterConfig:
		return []interface{}{c.Limit, c.SlidingWindowSize, c.BucketSize, now}
	default:
		return nil
	}
}
