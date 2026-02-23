package limiter

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
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

		results, err := e.Allow(r.Context(), key)
		if err != nil {
			var redisErr redis.Error
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.As(err, &redisErr) {
				if e.rateLimitConfig.FailOpen {
					e.log.Warn("fail_open_allowing_request", "key", key, "error", err, "reason", "redis_unavailable_or_timeout")
					next.ServeHTTP(w, r)
					return
				} else {
					e.log.Warn("request_blocked", "key", key, "path", r.URL.Path)

					w.Header().Set("X-RateLimit-Limit", strconv.Itoa(results.Limit))
					w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(results.Remaining))
					w.Header().Set("X-RateLimit-Reset", strconv.Itoa(results.Reset))

					http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
					// TODO: probably have a seperate metric for when redis goes unavailable
					e.middlewareMetrics.httpRequestRateLimitedTotal.WithLabelValues(r.URL.Path, r.Method, getClientType(r))
					return
				}
			}

			e.log.Error("rate_limit_check_failed", "key", key, "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !results.Allowed {
			e.log.Warn("request_blocked", "key", key, "path", r.URL.Path)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(results.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(results.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.Itoa(results.Reset))

			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			e.middlewareMetrics.httpRequestRateLimitedTotal.WithLabelValues(r.URL.Path, r.Method, getClientType(r))
			return
		}

		e.middlewareMetrics.httpRequestAllowedTotal.WithLabelValues(r.URL.Path, r.Method, getClientType(r))
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(results.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(results.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(results.Reset))

		e.log.Info("request_allowed", "key", key, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (se *SentinelEngine) PingRDB(ctx context.Context) error {
	return se.rdb.PingRDB(ctx)
}

func getClientKey(r *http.Request) string {
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "apikey:" + apiKey
	}
	return "ip:" + r.RemoteAddr
}
