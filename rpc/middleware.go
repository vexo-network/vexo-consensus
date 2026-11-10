package rpc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"mime"
	"net"
	"net/http"
	"net/http/pprof"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// maxRateLimitBuckets bounds the rate limiter's memory: one bucket per client key,
// and an IPv6 client is keyed by its /64, so a single host cannot mint keys.
const maxRateLimitBuckets = 1 << 16

// rateLimitSweepInterval is how often expired buckets are dropped. Expiry is otherwise
// lazy, so a request costs one map lookup instead of a scan of every bucket.
const rateLimitSweepInterval = time.Second

type rateBucket struct {
	WindowStart time.Time
	Count       int
}

type rateLimiter struct {
	mu        sync.Mutex
	now       func() time.Time
	window    time.Duration
	max       int
	buckets   map[string]rateBucket
	nextSweep time.Time
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
		if !unversionedPath(request.URL.Path) {
			writeError(writer, http.StatusNotFound, "endpoint not found: the API is served under "+stableAPIPrefix)
			return
		}
		handler.ServeHTTP(writer, request)
	})
}

// unversionedPath reports whether path is served without the version prefix: the probes, the
// Prometheus text metrics and the profiler, whose paths scrapers and orchestrators fix, and the
// Web3 JSON-RPC endpoints, whose paths Ethereum clients fix. Every other endpoint is an API of
// stableAPIPrefix only.
func unversionedPath(path string) bool {
	switch path {
	case "/", "/healthz", "/readyz", "/metrics/text", "/web3":
		return true
	}
	return strings.HasPrefix(path, "/debug/pprof/")
}

// registerPprofHandlers serves the profiler behind the admin token: its cmdline and heap
// endpoints expose process arguments and memory, which include secrets.
func registerPprofHandlers(mux *http.ServeMux, cfg Config) {
	register := func(path string, handler http.HandlerFunc) {
		mux.HandleFunc(path, func(writer http.ResponseWriter, request *http.Request) {
			if !allowAdmin(writer, request, cfg, "pprof") {
				return
			}
			handler(writer, request)
		})
	}
	register("/debug/pprof/", pprof.Index)
	register("/debug/pprof/cmdline", pprof.Cmdline)
	register("/debug/pprof/profile", pprof.Profile)
	register("/debug/pprof/symbol", pprof.Symbol)
	register("/debug/pprof/trace", pprof.Trace)
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

// cors adds CORS headers only for origins the operator listed in CORSAllowedOrigins. With
// no list a browser page gets no cross-origin access at all. A WebSocket handshake is not
// subject to CORS, so one from an unlisted origin is refused here instead.
func cors(next http.Handler, cfg Config) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := strings.TrimSpace(request.Header.Get("Origin"))
		allowedOrigin, allowed := corsAllowedOrigin(origin, cfg)
		if origin != "" {
			writer.Header().Add("Vary", "Origin")
		}
		if allowed {
			writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			writer.Header().Set("Access-Control-Max-Age", "600")
			writer.Header().Add("Vary", "Access-Control-Request-Method")
			writer.Header().Add("Vary", "Access-Control-Request-Headers")
		} else if origin != "" && isWebSocketUpgrade(request) {
			writeJSON(writer, http.StatusForbidden, map[string]string{"error": "websocket origin is not allowed"})
			return
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// corsAllowedOrigin reports the Access-Control-Allow-Origin value for origin: the origin
// itself when it is listed, "*" when the operator listed the wildcard.
func corsAllowedOrigin(origin string, cfg Config) (string, bool) {
	if origin == "" {
		return "", false
	}
	for _, allowed := range cfg.CORSAllowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "*" {
			return "*", true
		}
		if allowed == origin {
			return origin, true
		}
	}
	return "", false
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

// Allow counts the request against its client's window. Expired buckets are dropped by a
// sweep at most once per rateLimitSweepInterval, and a client that has no bucket while the
// table is full is refused until the sweep frees room.
func (limiter *rateLimiter) Allow(request *http.Request) bool {
	key := clientKey(request)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.now()
	if !now.Before(limiter.nextSweep) {
		for bucketKey, bucket := range limiter.buckets {
			if now.Sub(bucket.WindowStart) >= limiter.window {
				delete(limiter.buckets, bucketKey)
			}
		}
		limiter.nextSweep = now.Add(min(limiter.window, rateLimitSweepInterval))
	}
	bucket, found := limiter.buckets[key]
	if !found && len(limiter.buckets) >= maxRateLimitBuckets {
		return false
	}
	if !found || now.Sub(bucket.WindowStart) >= limiter.window {
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

// clientKey identifies the rate-limited client: its IPv4 address, or the /64 of its IPv6
// address, since one host routinely controls a whole /64.
func clientKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	if host == "" {
		return "unknown"
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	address = address.WithZone("").Unmap()
	if address.Is6() {
		prefix, err := address.Prefix(64)
		if err == nil {
			return prefix.String()
		}
	}
	return address.String()
}

func allowGet(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet {
		return true
	}
	writer.Header().Set("Allow", http.MethodGet)
	writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

// allowPost admits state-changing requests: POST with a JSON body. Requiring the media type
// keeps a cross-site form or text/plain request, which a browser sends without a preflight,
// from reaching a handler.
func allowPost(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return false
	}
	return requireJSONContentType(writer, request)
}

func requireJSONContentType(writer http.ResponseWriter, request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err == nil && mediaType == "application/json" {
		return true
	}
	writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"error": "content type must be application/json"})
	return false
}

// allowAdmin admits a request to an admin endpoint. It fails closed: with no admin token
// configured nothing is authorized. The Host must be an IP literal, localhost or an
// AllowedHosts entry, so a page cannot reach the endpoint through a rebound DNS name.
func allowAdmin(writer http.ResponseWriter, request *http.Request, cfg Config, scope string) bool {
	if !adminHostAllowed(request.Host, cfg) {
		auditAdmin(cfg, request, scope, false, "invalid_host")
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "admin request host is not allowed"})
		return false
	}
	if !adminConfigured(cfg) {
		auditAdmin(cfg, request, scope, false, "not_configured")
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "admin endpoints are disabled until an admin token is configured"})
		return false
	}
	token, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		auditAdmin(cfg, request, scope, false, "missing_bearer")
		writer.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "admin authorization is required"})
		return false
	}
	if !adminTokenAllowed(cfg, token, scope) {
		auditAdmin(cfg, request, scope, false, "invalid_or_insufficient_scope")
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "admin authorization is invalid"})
		return false
	}
	auditAdmin(cfg, request, scope, true, "")
	return true
}

// allowAdminPost admits a state-changing admin request: authorized for the scope, POST,
// and a JSON body. Authorization comes first so an unauthenticated caller learns nothing
// about the endpoint.
func allowAdminPost(writer http.ResponseWriter, request *http.Request, cfg Config, scope string) bool {
	return allowAdmin(writer, request, cfg, scope) && allowPost(writer, request)
}

func adminConfigured(cfg Config) bool {
	if cfg.AdminToken != "" {
		return true
	}
	for token := range cfg.AdminTokens {
		if token != "" {
			return true
		}
	}
	return false
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

// adminHostAllowed reports whether the Host header names this server in a way a rebinding
// attacker cannot: an IP literal (the browser connected to that address, so its origin is
// that address), localhost, or a name the operator listed.
func adminHostAllowed(hostport string, cfg Config) bool {
	host := hostport
	if withoutPort, _, err := net.SplitHostPort(hostport); err == nil {
		host = withoutPort
	}
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	if host == "" {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil || host == "localhost" {
		return true
	}
	for _, allowed := range cfg.AllowedHosts {
		allowed = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(allowed)), ".")
		if allowed == "*" || allowed == host {
			return true
		}
	}
	return false
}

// adminTokenAllowed compares digests of the tokens, so neither the token length nor the
// position of a match shows in the timing, and checks every configured token.
func adminTokenAllowed(cfg Config, token string, scope string) bool {
	if token == "" {
		return false
	}
	presented := sha256.Sum256([]byte(token))
	allowed := 0
	if cfg.AdminToken != "" {
		configured := sha256.Sum256([]byte(cfg.AdminToken))
		allowed |= subtle.ConstantTimeCompare(presented[:], configured[:])
	}
	for configuredToken, scopes := range cfg.AdminTokens {
		if configuredToken == "" {
			continue
		}
		configured := sha256.Sum256([]byte(configuredToken))
		if subtle.ConstantTimeCompare(presented[:], configured[:]) == 1 && adminScopeAllowed(scopes, scope) {
			allowed = 1
		}
	}
	return allowed == 1
}

func adminScopeAllowed(scopes []string, scope string) bool {
	for _, candidate := range scopes {
		if candidate == "*" || candidate == scope {
			return true
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
