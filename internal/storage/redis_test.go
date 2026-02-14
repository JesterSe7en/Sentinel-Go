package storage

import (
	"testing"
)

func TestGetAlgorithmNames(t *testing.T) {
	algos := GetAlgorithmNames()

	expected := []string{
		"TokenBucket",
		"LeakyBucket",
		"FixedWindow",
		"SlidingWindowLog",
		"SlidingWindowCounter",
	}

	if len(algos) != len(expected) {
		t.Errorf("expected %d algorithms, got %d", len(expected), len(algos))
	}

	for i, expectedAlgo := range expected {
		if i >= len(algos) {
			break
		}
		if algos[i] != expectedAlgo {
			t.Errorf("expected %q at index %d, got %q", expectedAlgo, i, algos[i])
		}
	}
}

func TestScriptsLoaded(t *testing.T) {
	expectedScripts := []string{
		"TokenBucket",
		"LeakyBucket",
		"FixedWindow",
		"SlidingWindowLog",
		"SlidingWindowCounter",
	}

	for _, scriptName := range expectedScripts {
		if _, exists := Scripts[scriptName]; !exists {
			t.Errorf("expected script %q to be loaded, but it was not found", scriptName)
		}
	}
}

func TestAlgoRegistry(t *testing.T) {
	expectedRegistry := map[string]string{
		"TokenBucket":          "algos/token_bucket.lua",
		"LeakyBucket":          "algos/leaky_bucket.lua",
		"FixedWindow":          "algos/fixed_window.lua",
		"SlidingWindowLog":     "algos/sliding_window_log.lua",
		"SlidingWindowCounter": "algos/sliding_window_counter.lua",
	}

	for algoName, expectedPath := range expectedRegistry {
		actualPath, exists := algoRegistry[algoName]
		if !exists {
			t.Errorf("expected algorithm %q in registry, but it was not found", algoName)
			continue
		}
		if actualPath != expectedPath {
			t.Errorf("expected path %q for algorithm %q, got %q", expectedPath, algoName, actualPath)
		}
	}
}
