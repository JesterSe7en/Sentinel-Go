package storage

import (
	"context"
	"embed"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Basically we are embedding the raw binary into the go project
// Use the embed directive to pull in all .lua files in the lua folder
//
//go:embed algos/*.lua
var luaScripts embed.FS

var Scripts = make(map[string]*redis.Script)

// Algorithm registry - SINGLE PLACE to add new algorithms
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

func init() {
	for algoName, scriptPath := range algoRegistry {
		src, err := luaScripts.ReadFile(scriptPath)
		if err != nil {
			panic(fmt.Sprintf("failed to read embedded script %s: %v", scriptPath, err))
		}
		Scripts[algoName] = redis.NewScript(string(src))
	}
}

func NewRedisClient(masterName string, sentinels []string, password string, db int) *redis.Client {
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

	return rdb

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
