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
	handler = cors(handler, cfg)
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

func cors(next http.Handler, cfg Config) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := strings.TrimSpace(request.Header.Get("Origin"))
		if origin != "" {
			writer.Header().Set("Access-Control-Allow-Origin", corsAllowedOrigin(origin, cfg))
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			if strings.EqualFold(strings.TrimSpace(request.Header.Get("Access-Control-Request-Private-Network")), "true") {
				writer.Header().Set("Access-Control-Allow-Private-Network", "true")
				writer.Header().Add("Vary", "Access-Control-Request-Private-Network")
			}
			writer.Header().Set("Access-Control-Max-Age", "600")
			writer.Header().Add("Vary", "Origin")
			writer.Header().Add("Vary", "Access-Control-Request-Method")
			writer.Header().Add("Vary", "Access-Control-Request-Headers")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func corsAllowedOrigin(origin string, cfg Config) string {
	if len(cfg.CORSAllowedOrigins) == 0 {
		return "*"
	}
	for _, allowed := range cfg.CORSAllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" || allowed == origin {
			return allowed
		}
	}
	return cfg.CORSAllowedOrigins[0]
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

func allowAdmin(writer http.ResponseWriter, request *http.Request, cfg Config, scope string) bool {
	if cfg.AdminToken == "" && len(cfg.AdminTokens) == 0 {
		auditAdmin(cfg, request, scope, true, "not_configured_open")
		return true
	}
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		auditAdmin(cfg, request, scope, false, "missing_bearer")
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "admin authorization is required"})
		return false
	}
	token := strings.TrimPrefix(header, prefix)
	if !adminTokenAllowed(cfg, token, scope) {
		auditAdmin(cfg, request, scope, false, "invalid_or_insufficient_scope")
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "admin authorization is invalid"})
		return false
	}
	auditAdmin(cfg, request, scope, true, "")
	return true
}

func adminTokenAllowed(cfg Config, token string, scope string) bool {
	if cfg.AdminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) == 1 {
		return true
	}
	for configuredToken, scopes := range cfg.AdminTokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(configuredToken)) != 1 {
			continue
		}
		for _, candidate := range scopes {
			if candidate == "*" || candidate == scope {
				return true
			}
		}
	}
	return false
}

func auditAdmin(cfg Config, request *http.Request, scope string, authorized bool, reason string) {
	if cfg.AdminAuditSink == nil {
		return
	}
	cfg.AdminAuditSink(AdminAuditEvent{
		Scope:      scope,
		Path:       request.URL.Path,
		Method:     request.Method,
		RemoteAddr: request.RemoteAddr,
		Authorized: authorized,
		Reason:     reason,
		At:         time.Now(),
	})
}
