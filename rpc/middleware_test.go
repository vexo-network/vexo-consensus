package rpc

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func adminRequest(method string, host string, headers map[string]string) *http.Request {
	request := httptest.NewRequest(method, "/prune", strings.NewReader(`{}`))
	request.Host = host
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

func TestAdminFailsClosedWithoutConfiguredToken(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no token":           {},
		"empty scoped token": {AdminTokens: map[string][]string{"": {"*"}}},
	} {
		var events []AdminAuditEvent
		cfg.AdminAuditSink = func(event AdminAuditEvent) { events = append(events, event) }
		for _, headers := range []map[string]string{nil, {"Authorization": "Bearer "}, {"Authorization": "Bearer anything"}} {
			response := httptest.NewRecorder()
			if allowAdmin(response, adminRequest(http.MethodPost, "127.0.0.1:26657", headers), cfg, "prune") {
				t.Fatalf("%s: admin request was authorized without a configured token", name)
			}
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s: expected 401, got %d", name, response.Code)
			}
		}
		if len(events) != 3 || events[0].Authorized || events[0].Reason != "not_configured" {
			t.Fatalf("%s: unexpected audit events %+v", name, events)
		}
	}
}

func TestAdminRequiresMatchingBearerToken(t *testing.T) {
	cfg := Config{
		AdminToken:  "root",
		AdminTokens: map[string][]string{"pruner": {"prune"}, "everything": {"*"}},
	}
	cases := []struct {
		name     string
		header   string
		scope    string
		expected int
	}{
		{"missing header", "", "prune", http.StatusUnauthorized},
		{"not a bearer credential", "Basic root", "prune", http.StatusUnauthorized},
		{"empty bearer", "Bearer ", "prune", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", "prune", http.StatusForbidden},
		{"token with trailing space", "Bearer root ", "prune", http.StatusForbidden},
		{"prefix of token", "Bearer roo", "prune", http.StatusForbidden},
		{"root token", "Bearer root", "prune", http.StatusOK},
		{"lowercase scheme", "bearer root", "replay", http.StatusOK},
		{"scoped token in scope", "Bearer pruner", "prune", http.StatusOK},
		{"scoped token out of scope", "Bearer pruner", "replay", http.StatusForbidden},
		{"wildcard scoped token", "Bearer everything", "consensus", http.StatusOK},
	}
	for _, testCase := range cases {
		response := httptest.NewRecorder()
		headers := map[string]string{}
		if testCase.header != "" {
			headers["Authorization"] = testCase.header
		}
		allowed := allowAdmin(response, adminRequest(http.MethodPost, "localhost:26657", headers), cfg, testCase.scope)
		status := response.Code
		if allowed {
			status = http.StatusOK
		}
		if status != testCase.expected {
			t.Fatalf("%s: expected %d, got %d", testCase.name, testCase.expected, status)
		}
	}
	response := httptest.NewRecorder()
	allowAdmin(response, adminRequest(http.MethodPost, "localhost", nil), cfg, "prune")
	if response.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("expected a bearer challenge, got %q", response.Header().Get("WWW-Authenticate"))
	}
}

func TestAdminChecksHostBeforeToken(t *testing.T) {
	cfg := Config{AdminToken: "root", AllowedHosts: []string{"Node.Example.com"}}
	authorized := map[string]string{"Authorization": "Bearer root"}
	for host, expected := range map[string]bool{
		"127.0.0.1:26657":         true,
		"[::1]:26657":             true,
		"[::1]":                   true,
		"192.0.2.10":              true,
		"localhost":               true,
		"LOCALHOST:26657":         true,
		"node.example.com":        true,
		"node.example.com:443":    true,
		"node.example.com.":       true,
		"rebind.attacker.test":    false,
		"rebind.attacker.test:1":  false,
		"example.com":             false,
		"notlocalhost":            false,
		"localhost.attacker.test": false,
		"":                        false,
	} {
		response := httptest.NewRecorder()
		if got := allowAdmin(response, adminRequest(http.MethodPost, host, authorized), cfg, "prune"); got != expected {
			t.Fatalf("host %q: expected allowed=%t, got %t (status %d)", host, expected, got, response.Code)
		}
		if !expected && response.Code != http.StatusForbidden {
			t.Fatalf("host %q: expected 403, got %d", host, response.Code)
		}
	}
	wildcard := Config{AdminToken: "root", AllowedHosts: []string{"*"}}
	if !allowAdmin(httptest.NewRecorder(), adminRequest(http.MethodPost, "anything.test", authorized), wildcard, "prune") {
		t.Fatal("expected the wildcard to allow any host")
	}
}

func TestAllowAdminPostRequiresAuthorizationBeforeMethodAndMediaType(t *testing.T) {
	cfg := Config{AdminToken: "root"}
	json := map[string]string{"Authorization": "Bearer root", "Content-Type": "application/json"}

	response := httptest.NewRecorder()
	if allowAdminPost(response, adminRequest(http.MethodGet, "localhost", nil), cfg, "prune") || response.Code != http.StatusUnauthorized {
		t.Fatalf("expected an unauthenticated GET to be answered 401, got %d", response.Code)
	}
	response = httptest.NewRecorder()
	if allowAdminPost(response, adminRequest(http.MethodGet, "localhost", json), cfg, "prune") || response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected an authorized GET to be answered 405, got %d", response.Code)
	}
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x", "application/jsonp", "json"} {
		headers := map[string]string{"Authorization": "Bearer root"}
		if contentType != "" {
			headers["Content-Type"] = contentType
		}
		response = httptest.NewRecorder()
		if allowAdminPost(response, adminRequest(http.MethodPost, "localhost", headers), cfg, "prune") || response.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("content type %q: expected 415, got %d", contentType, response.Code)
		}
	}
	for _, contentType := range []string{"application/json", "application/json; charset=utf-8", "Application/JSON"} {
		headers := map[string]string{"Authorization": "Bearer root", "Content-Type": contentType}
		if !allowAdminPost(httptest.NewRecorder(), adminRequest(http.MethodPost, "localhost", headers), cfg, "prune") {
			t.Fatalf("content type %q: expected the request to be admitted", contentType)
		}
	}
}

func TestPprofRequiresAdminToken(t *testing.T) {
	mux := http.NewServeMux()
	registerPprofHandlers(mux, Config{AdminToken: "root"})
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/symbol"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Host = "127.0.0.1"
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401 without a token, got %d", path, response.Code)
		}
		request.Header.Set("Authorization", "Bearer root")
		response = httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: expected 200 with a token, got %d", path, response.Code)
		}
	}
	unconfigured := http.NewServeMux()
	registerPprofHandlers(unconfigured, Config{})
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	request.Host = "127.0.0.1"
	response := httptest.NewRecorder()
	unconfigured.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected pprof to stay closed without an admin token, got %d", response.Code)
	}
}

func TestCORSSendsNothingByDefault(t *testing.T) {
	reached := 0
	handler := applyMiddleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		reached++
		writer.WriteHeader(http.StatusOK)
	}), Config{})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
		request := httptest.NewRequest(method, "/web3", nil)
		request.Header.Set("Origin", "https://evil.example")
		request.Header.Set("Access-Control-Request-Method", "POST")
		request.Header.Set("Access-Control-Request-Private-Network", "true")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		for name := range response.Header() {
			if strings.HasPrefix(name, "Access-Control-") {
				t.Fatalf("%s: unexpected CORS header %s=%q", method, name, response.Header().Get(name))
			}
		}
	}
	if reached != 2 {
		t.Fatalf("expected the GET and POST to reach the handler and the preflight not to, got %d", reached)
	}
}

func TestCORSAllowsOnlyListedOrigins(t *testing.T) {
	handler := applyMiddleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}), Config{CORSAllowedOrigins: []string{"https://remix.ethereum.org", " https://app.example "}})
	cases := map[string]string{
		"https://remix.ethereum.org":              "https://remix.ethereum.org",
		"https://app.example":                     "https://app.example",
		"https://evil.example":                    "",
		"https://remix.ethereum.org.evil.example": "",
		"null": "",
	}
	for origin, expected := range cases {
		request := httptest.NewRequest(http.MethodOptions, "/web3", nil)
		request.Header.Set("Origin", origin)
		request.Header.Set("Access-Control-Request-Method", "POST")
		request.Header.Set("Access-Control-Request-Private-Network", "true")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Header().Get("Access-Control-Allow-Origin"); got != expected {
			t.Fatalf("origin %q: expected allow-origin %q, got %q", origin, expected, got)
		}
		if got := response.Header().Get("Access-Control-Allow-Private-Network"); got != "" {
			t.Fatalf("origin %q: private network access must never be granted, got %q", origin, got)
		}
		if expected == "" && response.Header().Get("Access-Control-Allow-Methods") != "" {
			t.Fatalf("origin %q: unlisted origin received CORS headers", origin)
		}
		if !strings.Contains(strings.Join(response.Header().Values("Vary"), ","), "Origin") {
			t.Fatalf("origin %q: response must vary on Origin", origin)
		}
	}
	wildcard := applyMiddleware(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}), Config{CORSAllowedOrigins: []string{"*"}})
	request := httptest.NewRequest(http.MethodGet, "/web3", nil)
	request.Header.Set("Origin", "https://anything.example")
	response := httptest.NewRecorder()
	wildcard.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected the configured wildcard, got %q", got)
	}
}

func TestWebSocketHandshakeFromUnlistedOriginIsRefused(t *testing.T) {
	reached := 0
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { reached++ })
	upgrade := func(origin string) *http.Request {
		request := httptest.NewRequest(http.MethodGet, "/web3", nil)
		request.Header.Set("Upgrade", "websocket")
		request.Header.Set("Connection", "Upgrade")
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		return request
	}
	closed := applyMiddleware(next, Config{})
	response := httptest.NewRecorder()
	closed.ServeHTTP(response, upgrade("https://evil.example"))
	if response.Code != http.StatusForbidden || reached != 0 {
		t.Fatalf("expected a browser WebSocket handshake to be refused by default, got %d (reached=%d)", response.Code, reached)
	}
	closed.ServeHTTP(httptest.NewRecorder(), upgrade(""))
	if reached != 1 {
		t.Fatal("expected a non-browser client without an Origin to connect")
	}
	listed := applyMiddleware(next, Config{CORSAllowedOrigins: []string{"https://app.example"}})
	listed.ServeHTTP(httptest.NewRecorder(), upgrade("https://app.example"))
	if reached != 2 {
		t.Fatal("expected a listed origin to connect")
	}
	response = httptest.NewRecorder()
	listed.ServeHTTP(response, upgrade("https://evil.example"))
	if response.Code != http.StatusForbidden || reached != 2 {
		t.Fatalf("expected an unlisted origin to be refused, got %d (reached=%d)", response.Code, reached)
	}
}

func rateLimitRequest(remoteAddr string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/status", nil)
	request.RemoteAddr = remoteAddr
	return request
}

func newTestLimiter(window time.Duration, max int) (*rateLimiter, *time.Time) {
	now := time.Unix(1_000_000, 0)
	limiter := newRateLimiter(window, max)
	limiter.now = func() time.Time { return now }
	return limiter, &now
}

func TestRateLimiterKeysIPv6ByPrefix(t *testing.T) {
	limiter, _ := newTestLimiter(time.Hour, 1)
	if !limiter.Allow(rateLimitRequest("[2001:db8:1:2::1]:1000")) {
		t.Fatal("expected the first request to pass")
	}
	for _, sibling := range []string{"[2001:db8:1:2::2]:1", "[2001:db8:1:2:ffff:ffff:ffff:ffff]:2", "[2001:db8:1:2::1%eth0]:3"} {
		if limiter.Allow(rateLimitRequest(sibling)) {
			t.Fatalf("expected %s to share the /64 bucket", sibling)
		}
	}
	if !limiter.Allow(rateLimitRequest("[2001:db8:1:3::1]:1")) {
		t.Fatal("expected another /64 to have its own bucket")
	}
	if !limiter.Allow(rateLimitRequest("192.0.2.1:1")) || limiter.Allow(rateLimitRequest("192.0.2.1:2")) {
		t.Fatal("expected IPv4 clients to be keyed per address")
	}
	if !limiter.Allow(rateLimitRequest("192.0.2.2:1")) {
		t.Fatal("expected another IPv4 address to have its own bucket")
	}
	if limiter.Allow(rateLimitRequest("[::ffff:192.0.2.1]:3")) {
		t.Fatal("expected an IPv4-mapped IPv6 address to share the IPv4 bucket")
	}
}

func TestRateLimiterBoundsBucketsAndRecoversAfterWindow(t *testing.T) {
	limiter, now := newTestLimiter(time.Minute, 2)
	addr := func(index int) string { return fmt.Sprintf("10.%d.%d.%d:1", index>>16, (index>>8)&0xff, index&0xff) }
	for index := 0; index < maxRateLimitBuckets; index++ {
		if !limiter.Allow(rateLimitRequest(addr(index))) {
			t.Fatalf("client %d should have been admitted", index)
		}
	}
	if len(limiter.buckets) != maxRateLimitBuckets {
		t.Fatalf("expected a full table, got %d", len(limiter.buckets))
	}
	if limiter.Allow(rateLimitRequest("203.0.113.1:1")) {
		t.Fatal("expected a new client to be refused while the table is full")
	}
	if len(limiter.buckets) != maxRateLimitBuckets {
		t.Fatalf("expected the table to stay at its cap, got %d", len(limiter.buckets))
	}
	if !limiter.Allow(rateLimitRequest(addr(0))) {
		t.Fatal("expected a tracked client to keep its remaining budget")
	}
	if limiter.Allow(rateLimitRequest(addr(0))) {
		t.Fatal("expected a tracked client to be limited")
	}
	*now = now.Add(2 * time.Minute)
	if !limiter.Allow(rateLimitRequest("203.0.113.1:1")) {
		t.Fatal("expected the expired table to make room for a new client")
	}
	if len(limiter.buckets) != 1 {
		t.Fatalf("expected the sweep to drop every expired bucket, got %d", len(limiter.buckets))
	}
}

func TestRateLimiterExpiresBucketsLazily(t *testing.T) {
	limiter, now := newTestLimiter(time.Hour, 1)
	limiter.Allow(rateLimitRequest("192.0.2.1:1"))
	limiter.buckets["stale"] = rateBucket{WindowStart: now.Add(-2 * time.Hour), Count: 1}
	*now = now.Add(rateLimitSweepInterval / 2)
	limiter.Allow(rateLimitRequest("192.0.2.2:1"))
	if _, present := limiter.buckets["stale"]; !present {
		t.Fatal("expected a request inside the sweep interval not to scan the table")
	}
	*now = now.Add(rateLimitSweepInterval)
	limiter.Allow(rateLimitRequest("192.0.2.3:1"))
	if _, present := limiter.buckets["stale"]; present {
		t.Fatal("expected the next sweep to drop the expired bucket")
	}
	if len(limiter.buckets) != 3 {
		t.Fatalf("expected live buckets to survive the sweep, got %d", len(limiter.buckets))
	}
}
