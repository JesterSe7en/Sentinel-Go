package limiter

import (
	"testing"
	"time"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/config"
)

func TestSentinelEngine_BuildArgsFromConfig_TokenBucket(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{
			TokenBucket: &config.TokenBucketConfig{
				Capacity:   10,
				RefillRate: 5.0,
			},
		},
	}

	args := engine.buildArgsFromConfig(algorithm.AlgorithmTokenBucket, engine.rateLimitConfig.TokenBucket)
	if args == nil {
		t.Fatal("expected args, got nil")
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
	if args[0] != 10 {
		t.Errorf("expected capacity 10, got %v", args[0])
	}
	if args[1] != 5.0 {
		t.Errorf("expected refill rate 5.0, got %v", args[1])
	}
}

func TestSentinelEngine_BuildArgsFromConfig_LeakyBucket(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{
			LeakyBucket: &config.LeakyBucketConfig{
				Capacity: 100,
				LeakRate: 10.0,
			},
		},
	}

	args := engine.buildArgsFromConfig(algorithm.AlgorithmLeakyBucket, engine.rateLimitConfig.LeakyBucket)
	if args == nil {
		t.Fatal("expected args, got nil")
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
	if args[0] != 100 {
		t.Errorf("expected capacity 100, got %v", args[0])
	}
	if args[1] != 10.0 {
		t.Errorf("expected leak rate 10.0, got %v", args[1])
	}
}

func TestSentinelEngine_BuildArgsFromConfig_FixedWindow(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{
			FixedWindow: &config.FixedWindowConfig{
				Limit:  50,
				Window: 30 * time.Second,
			},
		},
	}

	args := engine.buildArgsFromConfig(algorithm.AlgorithmFixedWindow, engine.rateLimitConfig.FixedWindow)
	if args == nil {
		t.Fatal("expected args, got nil")
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
	if args[0] != 50 {
		t.Errorf("expected limit 50, got %v", args[0])
	}
	if args[1] != 30 {
		t.Errorf("expected window 30, got %v", args[1])
	}
}

func TestSentinelEngine_BuildArgsFromConfig_SlidingWindowLog(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{
			SlidingWindowLog: &config.SlidingWindowLogConfig{
				Limit:             100,
				SlidingWindowSize: 60,
			},
		},
	}

	args := engine.buildArgsFromConfig(algorithm.AlgorithmSlidingWindowLog, engine.rateLimitConfig.SlidingWindowLog)
	if args == nil {
		t.Fatal("expected args, got nil")
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
	if args[0] != 100 {
		t.Errorf("expected limit 100, got %v", args[0])
	}
	if args[1] != 60 {
		t.Errorf("expected window size 60, got %v", args[1])
	}
}

func TestSentinelEngine_BuildArgsFromConfig_SlidingWindowCounter(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{
			SlidingWindowCounter: &config.SlidingWindowCounterConfig{
				Limit:             100,
				SlidingWindowSize: 60,
				BucketSize:        10,
			},
		},
	}

	args := engine.buildArgsFromConfig(algorithm.AlgorithmSlidingWindowCounter, engine.rateLimitConfig.SlidingWindowCounter)
	if args == nil {
		t.Fatal("expected args, got nil")
	}
	if len(args) != 4 {
		t.Errorf("expected 4 args, got %d", len(args))
	}
	if args[0] != 100 {
		t.Errorf("expected limit 100, got %v", args[0])
	}
	if args[1] != 60 {
		t.Errorf("expected window size 60, got %v", args[1])
	}
	if args[2] != 10 {
		t.Errorf("expected bucket size 10, got %v", args[2])
	}
}

func TestSentinelEngine_BuildArgsFromConfig_NilConfig(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{},
	}

	args := engine.buildArgsFromConfig(algorithm.AlgorithmTokenBucket, nil)
	if args != nil {
		t.Errorf("expected nil for nil config, got %v", args)
	}
}

func TestSentinelEngine_BuildArgsFromConfig_UnknownType(t *testing.T) {
	engine := &SentinelEngine{
		rateLimitConfig: &config.RateLimitConfig{},
	}

	type unknownConfig struct {
		Value string
	}
	args := engine.buildArgsFromConfig(algorithm.AlgorithmTokenBucket, &unknownConfig{Value: "test"})
	if args != nil {
		t.Errorf("expected nil for unknown config type, got %v", args)
	}
}

func TestSentinelEngine_ListAlgorithm(t *testing.T) {
	engine := &SentinelEngine{}

	algos := engine.ListAlgorithm()
	if len(algos) != 5 {
		t.Errorf("expected 5 algorithms, got %d", len(algos))
	}
}
