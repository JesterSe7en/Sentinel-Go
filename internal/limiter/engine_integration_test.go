//go:build integration
// +build integration

package limiter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
	"github.com/JesterSe7en/Sentinel-Go/internal/logger"
	"github.com/JesterSe7en/Sentinel-Go/internal/storage"
	"github.com/joho/godotenv"
)

func init() {
	dir, _ := os.Getwd()
	envPath := filepath.Join(dir, "..", "..", ".env")
	godotenv.Load(envPath)
}

func TestNewSentinelEngine_Integration(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Skipping: failed to load config: %v", err)
	}

	log, _ := logger.New("", false, false)
	rdb := storage.NewRedisClient(
		cfg.RedisCfg.MasterName,
		cfg.RedisCfg.SentinelAddrs,
		cfg.RedisCfg.Password,
		cfg.RedisCfg.DB,
	)
	defer rdb.Close()

	engine, err := NewSentinelEngine(rdb, log, cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Test Allow
	allowed, err := engine.Allow(context.Background(), "test-key-integration")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}
	t.Logf("First request allowed: %v", allowed)

	// Test UpdateAlgorithm
	err = engine.UpdateAlgorithm(context.Background(), algorithm.AlgorithmLeakyBucket)
	if err != nil {
		t.Fatalf("UpdateAlgorithm failed: %v", err)
	}

	// Verify update
	algo, err := engine.GetCurrentAlgorithm(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentAlgorithm failed: %v", err)
	}
	if algo != "LeakyBucket" {
		t.Errorf("expected LeakyBucket, got %s", algo)
	}
}

func TestSentinelEngine_Allow_Integration(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Skipping: %v", err)
	}

	log, _ := logger.New("", false, false)
	rdb := storage.NewRedisClient(
		cfg.RedisCfg.MasterName,
		cfg.RedisCfg.SentinelAddrs,
		cfg.RedisCfg.Password,
		cfg.RedisCfg.DB,
	)
	defer rdb.Close()

	// Reset to TokenBucket for this test
	cfg.RateLimitCfg.Algorithm = "TokenBucket"

	engine, err := NewSentinelEngine(rdb, log, cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Make multiple requests
	for i := 0; i < 10; i++ {
		allowed, err := engine.Allow(context.Background(), "test-rate-limit-key")
		if err != nil {
			t.Fatalf("Allow failed on request %d: %v", i, err)
		}
		t.Logf("Request %d: allowed=%v", i+1, allowed)
	}
}

func TestSentinelEngine_ListAlgorithms_Integration(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Skipping: %v", err)
	}

	log, _ := logger.New("", false, false)
	rdb := storage.NewRedisClient(
		cfg.RedisCfg.MasterName,
		cfg.RedisCfg.SentinelAddrs,
		cfg.RedisCfg.Password,
		cfg.RedisCfg.DB,
	)
	defer rdb.Close()

	engine, err := NewSentinelEngine(rdb, log, cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	algos := engine.ListAlgorithm()
	if len(algos) != 5 {
		t.Errorf("expected 5 algorithms, got %d", len(algos))
	}
}

func TestSentinelEngine_UpdateAlgorithm_Integration(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Skipping: %v", err)
	}

	log, _ := logger.New("", false, false)
	rdb := storage.NewRedisClient(
		cfg.RedisCfg.MasterName,
		cfg.RedisCfg.SentinelAddrs,
		cfg.RedisCfg.Password,
		cfg.RedisCfg.DB,
	)
	defer rdb.Close()

	cfg.RateLimitCfg.Algorithm = "TokenBucket"

	engine, err := NewSentinelEngine(rdb, log, cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Test updating to all algorithms
	algorithms := []algorithm.RateLimitAlgorithm{
		algorithm.AlgorithmTokenBucket,
		algorithm.AlgorithmLeakyBucket,
		algorithm.AlgorithmFixedWindow,
		algorithm.AlgorithmSlidingWindowLog,
		algorithm.AlgorithmSlidingWindowCounter,
	}

	for _, algo := range algorithms {
		err := engine.UpdateAlgorithm(context.Background(), algo)
		if err != nil {
			t.Fatalf("UpdateAlgorithm failed for %s: %v", algo, err)
		}

		current, err := engine.GetCurrentAlgorithm(context.Background())
		if err != nil {
			t.Fatalf("GetCurrentAlgorithm failed: %v", err)
		}
		if current != algo.String() {
			t.Errorf("expected %s, got %s", algo, current)
		}
		t.Logf("Successfully updated to: %s", algo)
	}
}
