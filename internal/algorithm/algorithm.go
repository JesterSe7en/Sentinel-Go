package algorithm

import (
	"errors"
)

type RateLimitAlgorithm string

const (
	AlgorithmTokenBucket          RateLimitAlgorithm = "TokenBucket"
	AlgorithmLeakyBucket          RateLimitAlgorithm = "LeakyBucket"
	AlgorithmFixedWindow          RateLimitAlgorithm = "FixedWindow"
	AlgorithmSlidingWindowLog     RateLimitAlgorithm = "SlidingWindowLog"
	AlgorithmSlidingWindowCounter RateLimitAlgorithm = "SlidingWindowCounter"
)

var (
	ErrEmptyAlgorithm   = errors.New("algorithm: name cannot be empty")
	ErrUnknownAlgorithm = errors.New("algorithm: unknown algorithm")
)

func (a RateLimitAlgorithm) String() string {
	return string(a)
}

func (a RateLimitAlgorithm) IsValid() bool {
	switch a {
	case AlgorithmTokenBucket, AlgorithmLeakyBucket, AlgorithmFixedWindow, AlgorithmSlidingWindowLog, AlgorithmSlidingWindowCounter:
		return true
	default:
		return false
	}
}

func ParseAlgorithm(s string) (RateLimitAlgorithm, error) {
	if s == "" {
		return "", ErrEmptyAlgorithm
	}
	algo := RateLimitAlgorithm(s)
	if !algo.IsValid() {
		return "", ErrUnknownAlgorithm
	}
	return algo, nil
}
