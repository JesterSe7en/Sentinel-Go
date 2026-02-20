package limiter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type MiddlewareMetrics struct {
	httpRequestRateLimitedTotal *prometheus.CounterVec
	httpRequestAllowedTotal     *prometheus.CounterVec
}

func getClientType(r *http.Request) string {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "apikey"
	}
	return "ip"
}

func registerMiddlewareMetrics(reg prometheus.Registerer) *MiddlewareMetrics {
	return &MiddlewareMetrics{
		httpRequestRateLimitedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_request_rate_limit_total",
				Help: "Total number of rate limit checks performed.",
			},
			[]string{"endpoint", "method", "client_type"},
		),
		httpRequestAllowedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_request_allowed_total",
				Help: "Total number of requests allowed.",
			},
			[]string{"endpoint", "method", "client_type"},
		),
	}
}

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
			e.middlewareMetrics.httpRequestRateLimitedTotal.WithLabelValues(r.URL.Path, r.Method, getClientType(r))
			return
		}

		e.middlewareMetrics.httpRequestAllowedTotal.WithLabelValues(r.URL.Path, r.Method, getClientType(r))
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
