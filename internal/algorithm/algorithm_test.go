package algorithm

import (
	"errors"
	"testing"
)

func TestRateLimitAlgorithm_String(t *testing.T) {
	tests := []struct {
		name string
		algo RateLimitAlgorithm
		want string
	}{
		{"TokenBucket", AlgorithmTokenBucket, "TokenBucket"},
		{"LeakyBucket", AlgorithmLeakyBucket, "LeakyBucket"},
		{"FixedWindow", AlgorithmFixedWindow, "FixedWindow"},
		{"SlidingWindowLog", AlgorithmSlidingWindowLog, "SlidingWindowLog"},
		{"SlidingWindowCounter", AlgorithmSlidingWindowCounter, "SlidingWindowCounter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.algo.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimitAlgorithm_IsValid(t *testing.T) {
	tests := []struct {
		name string
		algo RateLimitAlgorithm
		want bool
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
			if got := tt.algo.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      RateLimitAlgorithm
		wantErr   bool
		wantErrIs error
	}{
		{"Valid_TokenBucket", "TokenBucket", AlgorithmTokenBucket, false, nil},
		{"Valid_LeakyBucket", "LeakyBucket", AlgorithmLeakyBucket, false, nil},
		{"Valid_FixedWindow", "FixedWindow", AlgorithmFixedWindow, false, nil},
		{"Valid_SlidingWindowLog", "SlidingWindowLog", AlgorithmSlidingWindowLog, false, nil},
		{"Valid_SlidingWindowCounter", "SlidingWindowCounter", AlgorithmSlidingWindowCounter, false, nil},
		{"Invalid_Empty", "", "", true, ErrEmptyAlgorithm},
		{"Invalid_Unknown", "UnknownAlgo", "", true, ErrUnknownAlgorithm},
		{"Invalid_InvalidName", "token_bucket", "", true, ErrUnknownAlgorithm},
		{"Invalid_Number", "123", "", true, ErrUnknownAlgorithm},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAlgorithm(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAlgorithm(%q) expected error, got nil", tt.input)
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("ParseAlgorithm(%q) error = %v, want %v", tt.input, err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseAlgorithm(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseAlgorithm(%q) = %v, want %v", tt.input, got, tt.wantErr)
			}
		})
	}
}
