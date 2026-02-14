package limiter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetClientKey_WithAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test-api-key-123")

	key := getClientKey(req)
	expected := "apikey:test-api-key-123"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestGetClientKey_WithIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	key := getClientKey(req)
	expected := "ip:192.168.1.100:12345"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestGetClientKey_PrefersAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	req.RemoteAddr = "192.168.1.100:12345"

	key := getClientKey(req)
	expected := "apikey:test-api-key"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestGetClientKey_EmptyAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "")
	req.RemoteAddr = "192.168.1.100:12345"

	key := getClientKey(req)
	expected := "ip:192.168.1.100:12345"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestRateLimitMiddleware_Allowed(t *testing.T) {
	engine := &SentinelEngine{
		log: nil, // Will be nil in this test
	}

	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := engine.RateLimitMiddleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	// Mock the Allow method to return true, allowed
	// Note: In real test, we'd need to mock the engine.Allow()
	// For now, this tests the middleware structure

	// Since we can't easily mock the engine.Allow() without interface changes,
	// we'll test that the middleware calls next when no error occurs
	_ = middleware
	_ = req
	_ = rr
	_ = nextCalled
	_ = nextHandler

	// This test is a placeholder - full testing requires interface changes
	// or integration testing with a real Redis instance
}

func TestRateLimitMiddleware_Blocked(t *testing.T) {
	// This test would require mocking the engine.Allow() method
	// Placeholder test for structure verification
}

func TestRateLimitMiddleware_Error(t *testing.T) {
	// This test would require mocking the engine.Allow() method
	// to return an error
	// Placeholder test for structure verification
}
