package rpc

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	WindowStart time.Time
	Count       int
}

type rateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	window  time.Duration
	max     int
	buckets map[string]rateBucket
}

func versionedHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == stableAPIPrefix {
			writeError(writer, http.StatusNotFound, "endpoint not found")
			return
		}
		if strings.HasPrefix(request.URL.Path, stableAPIPrefix+"/") {
			versionedRequest := request.Clone(request.Context())
			versionedRequest.URL.Path = strings.TrimPrefix(request.URL.Path, stableAPIPrefix)
			versionedRequest.URL.RawPath = ""
			writer.Header().Set("X-Vexo-RPC-Version", "v1")
			handler.ServeHTTP(writer, versionedRequest)
			return
		}
		handler.ServeHTTP(writer, request)
	})
}

func registerPprofHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func applyMiddleware(handler http.Handler, cfg Config) http.Handler {
	if cfg.RequestTimeout > 0 {
		handler = requestTimeout(handler, cfg.RequestTimeout)
	}
	if cfg.RateLimitMaxRequests > 0 {
		window := cfg.RateLimitWindow
		if window <= 0 {
			window = time.Second
		}
		handler = newRateLimiter(window, cfg.RateLimitMaxRequests).Handler(handler)
	}
	return handler
}

func requestTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), timeout)
		defer cancel()
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func newRateLimiter(window time.Duration, maxRequests int) *rateLimiter {
	return &rateLimiter{
		now:     time.Now,
		window:  window,
		max:     maxRequests,
		buckets: make(map[string]rateBucket),
	}
}

func (limiter *rateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !limiter.Allow(request) {
			writeJSON(writer, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (limiter *rateLimiter) Allow(request *http.Request) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.now()
	key := clientKey(request)
	for bucketKey, bucket := range limiter.buckets {
		if !bucket.WindowStart.IsZero() && now.Sub(bucket.WindowStart) >= limiter.window {
			delete(limiter.buckets, bucketKey)
		}
	}
	bucket := limiter.buckets[key]
	if bucket.WindowStart.IsZero() || now.Sub(bucket.WindowStart) >= limiter.window {
		limiter.buckets[key] = rateBucket{WindowStart: now, Count: 1}
		return true
	}
	if bucket.Count >= limiter.max {
		return false
	}
	bucket.Count++
	limiter.buckets[key] = bucket
	return true
}

func clientKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if request.RemoteAddr != "" {
		return request.RemoteAddr
	}
	return "unknown"
}

func allowGet(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet {
		return true
	}
	writer.Header().Set("Allow", http.MethodGet)
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func allowPost(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodPost {
		return true
	}
	writer.Header().Set("Allow", http.MethodPost)
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func allowAdmin(writer http.ResponseWriter, request *http.Request, adminToken string) bool {
	if adminToken == "" {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "admin authorization is not configured"})
		return false
	}
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "admin authorization is required"})
		return false
	}
	token := strings.TrimPrefix(header, prefix)
	if subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "admin authorization is invalid"})
		return false
	}
	return true
}
