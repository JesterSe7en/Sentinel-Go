package limiter

import (
	"net/http"
)

func (e *SentinelEngine) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := getClientKey(r)

		allowed, err := e.Allow(r.Context(), key)
		if err != nil {
			e.log.Error("rate_limit_check_failed", "key", key, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !allowed {
			e.log.Warn("request_blocked", "key", key, "path", r.URL.Path)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("429: Too Many Requests. Sentinel says no."))
			return
		}

		e.log.Info("request_allowed", "key", key, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func getClientKey(r *http.Request) string {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "apikey:" + apiKey
	}
	return "ip:" + r.RemoteAddr
}
