package algorithm

import (
	"testing"
)

func TestRateLimitAlgorithm_String(t *testing.T) {
	tests := []struct {
		name     string
		algo     RateLimitAlgorithm
		expected string
	}{
		{"TokenBucket", AlgorithmTokenBucket, "TokenBucket"},
		{"LeakyBucket", AlgorithmLeakyBucket, "LeakyBucket"},
		{"FixedWindow", AlgorithmFixedWindow, "FixedWindow"},
		{"SlidingWindowLog", AlgorithmSlidingWindowLog, "SlidingWindowLog"},
		{"SlidingWindowCounter", AlgorithmSlidingWindowCounter, "SlidingWindowCounter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.algo.String()
			if result != tt.expected {
				t.Errorf("String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRateLimitAlgorithm_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		algo     RateLimitAlgorithm
		expected bool
	}{
		{"Valid_TokenBucket", AlgorithmTokenBucket, true},
		{"Valid_LeakyBucket", AlgorithmLeakyBucket, true},
		{"Valid_FixedWindow", AlgorithmFixedWindow, true},
		{"Valid_SlidingWindowLog", AlgorithmSlidingWindowLog, true},
		{"Valid_SlidingWindowCounter", AlgorithmSlidingWindowCounter, true},
		{"Invalid_Empty", "", false},
		{"Invalid_Random", RateLimitAlgorithm("RandomAlgo"), false},
		{"Invalid_TokenBucketLowercase", RateLimitAlgorithm("tokenbucket"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.algo.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v for %v", result, tt.expected, tt.algo)
			}
		})
	}
}

func TestParseAlgorithm(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedAlgo RateLimitAlgorithm
		expectError  bool
	}{
		{"Valid_TokenBucket", "TokenBucket", AlgorithmTokenBucket, false},
		{"Valid_LeakyBucket", "LeakyBucket", AlgorithmLeakyBucket, false},
		{"Valid_FixedWindow", "FixedWindow", AlgorithmFixedWindow, false},
		{"Valid_SlidingWindowLog", "SlidingWindowLog", AlgorithmSlidingWindowLog, false},
		{"Valid_SlidingWindowCounter", "SlidingWindowCounter", AlgorithmSlidingWindowCounter, false},
		{"Invalid_Empty", "", "", true},
		{"Invalid_Unknown", "UnknownAlgo", "", true},
		{"Invalid_InvalidName", "token_bucket", "", true},
		{"Invalid_Number", "123", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAlgorithm(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseAlgorithm(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAlgorithm(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result != tt.expectedAlgo {
				t.Errorf("ParseAlgorithm(%q) = %v, want %v", tt.input, result, tt.expectedAlgo)
			}
		})
	}
}
