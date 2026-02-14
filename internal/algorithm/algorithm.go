package algorithm

import "fmt"

type RateLimitAlgorithm string

const (
	AlgorithmTokenBucket          RateLimitAlgorithm = "TokenBucket"
	AlgorithmLeakyBucket          RateLimitAlgorithm = "LeakyBucket"
	AlgorithmFixedWindow          RateLimitAlgorithm = "FixedWindow"
	AlgorithmSlidingWindowLog     RateLimitAlgorithm = "SlidingWindowLog"
	AlgorithmSlidingWindowCounter RateLimitAlgorithm = "SlidingWindowCounter"
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
	algo := RateLimitAlgorithm(s)
	if !algo.IsValid() {
		return "", fmt.Errorf("invalid algorithm: %s", s)
	}
	return algo, nil
}
