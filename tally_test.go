package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testConfig returns a Config suitable for use in tests.
func testConfig() *Config {
	return &Config{
		APIKey:     "test-api-key",
		MaxMetrics: 5,
	}
}

// resetStore re-initialises the global metrics store before each test that
// needs a clean slate.
func resetStore(maxMetrics int) {
	metrics = &MetricStore{
		entries:    make(map[string]MetricEntry),
		maxMetrics: maxMetrics,
	}
}

// push is a test helper that fires a POST /push with the given JSON body and
// API key against the provided handler, returning the recorded response.
func push(t *testing.T, handler http.Handler, body string, apiKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/push", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// ── Auth ─────────────────────────────────────────────────────────────────────

func TestAuth_CorrectKey(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"hits","value":1}`, cfg.APIKey)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuth_WrongKey(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"hits","value":1}`, "wrong-key")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_MissingKey(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"hits","value":1}`, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ── Metric name validation ────────────────────────────────────────────────────

func TestMetricName_Valid(t *testing.T) {
	cases := []string{
		"simple", "_underscore", "with_numbers_123", "A", "_",
	}
	for _, name := range cases {
		if !validateMetricName(name) {
			t.Errorf("expected valid name %q to pass", name)
		}
	}
}

func TestMetricName_Invalid(t *testing.T) {
	cases := []string{
		"",                    // empty
		"123starts_with_num",  // starts with digit
		"has-hyphen",          // hyphen not allowed
		"has space",           // space not allowed
		"has.dot",             // dot not allowed
		strings.Repeat("a", 201), // over 200 chars
	}
	for _, name := range cases {
		if validateMetricName(name) {
			t.Errorf("expected invalid name %q to fail", name)
		}
	}
}

func TestPush_InvalidMetricName(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"bad-name","value":1}`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPush_EmptyMetricName(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"","value":1}`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ── Metric value validation ───────────────────────────────────────────────────

func TestPush_InvalidValue_NaN(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	// JSON does not support NaN natively; send a string to force decode error.
	rr := push(t, h, `{"name":"m","value":"NaN"}`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ── Metric type validation ────────────────────────────────────────────────────

func TestPush_ValidTypes(t *testing.T) {
	for _, typ := range []string{"gauge", "counter", "histogram", "summary", "untyped", ""} {
		resetStore(100)
		cfg := testConfig()
		h := requireAPIKey(cfg, handlePush)
		body := fmt.Sprintf(`{"name":"m","value":1,"type":%q}`, typ)
		rr := push(t, h, body, cfg.APIKey)
		if rr.Code != http.StatusOK {
			t.Errorf("type %q: expected 200, got %d", typ, rr.Code)
		}
	}
}

func TestPush_InvalidType(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"m","value":1,"type":"bogus"}`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ── Label validation ──────────────────────────────────────────────────────────

func TestPush_ValidLabels(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"m","value":1,"labels":{"instance":"web-1","job":"cron"}}`, cfg.APIKey)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestPush_ReservedLabelPrefix(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"m","value":1,"labels":{"__reserved":"x"}}`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPush_InvalidLabelName(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{"name":"m","value":1,"labels":{"bad-name":"x"}}`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ── Empty / malformed body ────────────────────────────────────────────────────

func TestPush_EmptyBody(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, ``, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPush_MalformedJSON(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	rr := push(t, h, `{not valid json`, cfg.APIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ── Body size limit ───────────────────────────────────────────────────────────

func TestPush_BodyTooLarge(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	// Build a payload larger than the 64 KB limit by padding a label value.
	oversized := fmt.Sprintf(`{"name":"m","value":1,"labels":{"x":%q}}`, strings.Repeat("a", 70*1024))
	rr := push(t, h, oversized, cfg.APIKey)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 400 or 413, got %d", rr.Code)
	}
}

// ── Max metrics limit ─────────────────────────────────────────────────────────

func TestPush_MaxMetricsLimit(t *testing.T) {
	const limit = 3
	resetStore(limit)
	cfg := testConfig()
	cfg.MaxMetrics = limit
	metrics.maxMetrics = limit
	h := requireAPIKey(cfg, handlePush)

	// Fill to the limit — all should succeed.
	for i := 0; i < limit; i++ {
		body := fmt.Sprintf(`{"name":"metric_%d","value":%d}`, i, i)
		rr := push(t, h, body, cfg.APIKey)
		if rr.Code != http.StatusOK {
			t.Fatalf("push %d: expected 200, got %d", i, rr.Code)
		}
	}

	// One more new metric should be rejected.
	rr := push(t, h, `{"name":"overflow","value":1}`, cfg.APIKey)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

func TestPush_MaxMetrics_UpdateExistingAllowed(t *testing.T) {
	const limit = 1
	resetStore(limit)
	cfg := testConfig()
	cfg.MaxMetrics = limit
	metrics.maxMetrics = limit
	h := requireAPIKey(cfg, handlePush)

	// Fill to limit.
	rr := push(t, h, `{"name":"existing","value":1}`, cfg.APIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("first push: expected 200, got %d", rr.Code)
	}

	// Updating the same metric must succeed even though the store is full.
	rr = push(t, h, `{"name":"existing","value":2}`, cfg.APIKey)
	if rr.Code != http.StatusOK {
		t.Errorf("update existing: expected 200, got %d", rr.Code)
	}
}

// ── HTTP method enforcement ───────────────────────────────────────────────────

func TestMethodEnforcement(t *testing.T) {
	resetStore(100)
	cfg := testConfig()

	cases := []struct {
		method  string
		path    string
		handler http.Handler
		apiKey  string
		want    int
	}{
		{http.MethodPost, "/", http.HandlerFunc(handleIndex), "", http.StatusMethodNotAllowed},
		{http.MethodPost, "/health", http.HandlerFunc(handleHealth), "", http.StatusMethodNotAllowed},
		{http.MethodPost, "/metrics", http.HandlerFunc(handleMetrics), "", http.StatusMethodNotAllowed},
		// Auth middleware runs before the method check: no key → 401.
		{http.MethodGet, "/push", requireAPIKey(cfg, handlePush), "", http.StatusUnauthorized},
		// Valid key reaches handlePush which then rejects the wrong method → 405.
		{http.MethodGet, "/push", requireAPIKey(cfg, handlePush), cfg.APIKey, http.StatusMethodNotAllowed},
		{http.MethodPut, "/push", requireAPIKey(cfg, handlePush), cfg.APIKey, http.StatusMethodNotAllowed},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+tc.apiKey)
		}
		rr := httptest.NewRecorder()
		tc.handler.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Errorf("%s %s (key=%q): expected %d, got %d", tc.method, tc.path, tc.apiKey, tc.want, rr.Code)
		}
	}
}

// ── Unknown routes ────────────────────────────────────────────────────────────

func TestUnknownRoute_Returns404(t *testing.T) {
	for _, path := range []string{"/unknown", "/push/extra", "/foo/bar"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.URL.Path = path
		rr := httptest.NewRecorder()
		handleIndex(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("path %s: expected 404, got %d", path, rr.Code)
		}
	}
}

// ── Health endpoint ───────────────────────────────────────────────────────────

func TestHealth_Response(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected JSON content-type, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Errorf("expected body to contain \"ok\", got %q", rr.Body.String())
	}
}

// ── /metrics output ───────────────────────────────────────────────────────────

func TestMetrics_EmptyStore(t *testing.T) {
	resetStore(100)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "tally_up 1") {
		t.Errorf("expected tally_up 1 in output, got:\n%s", body)
	}
	if !strings.Contains(body, "# EOF") {
		t.Errorf("expected # EOF in empty output, got %q", body)
	}
}

func TestMetrics_ContainsTypeAndEOF(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	push(t, h, `{"name":"cpu_usage","value":72.4,"type":"gauge"}`, cfg.APIKey)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handleMetrics(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "# TYPE cpu_usage gauge") {
		t.Errorf("missing # TYPE line, got:\n%s", body)
	}
	if !strings.Contains(body, "cpu_usage 72.4") {
		t.Errorf("missing value line, got:\n%s", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "# EOF") {
		t.Errorf("missing # EOF at end, got:\n%s", body)
	}
}

func TestMetrics_LabelledSeriesRendered(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	push(t, h, `{"name":"cpu","value":10,"labels":{"instance":"web-1"}}`, cfg.APIKey)
	push(t, h, `{"name":"cpu","value":20,"labels":{"instance":"web-2"}}`, cfg.APIKey)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handleMetrics(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `cpu{instance="web-1"}`) {
		t.Errorf("missing web-1 series, got:\n%s", body)
	}
	if !strings.Contains(body, `cpu{instance="web-2"}`) {
		t.Errorf("missing web-2 series, got:\n%s", body)
	}
	// Both share one family — only one # TYPE line.
	if count := strings.Count(body, "# TYPE cpu"); count != 1 {
		t.Errorf("expected exactly 1 # TYPE cpu line, got %d", count)
	}
}

func TestMetrics_HistogramFamilyGrouped(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	push(t, h, `{"name":"req_duration_bucket","value":5,"type":"histogram"}`, cfg.APIKey)
	push(t, h, `{"name":"req_duration_sum","value":1.5,"type":"histogram"}`, cfg.APIKey)
	push(t, h, `{"name":"req_duration_count","value":5,"type":"histogram"}`, cfg.APIKey)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handleMetrics(rr, req)

	body := rr.Body.String()
	if count := strings.Count(body, "# TYPE req_duration histogram"); count != 1 {
		t.Errorf("expected exactly 1 # TYPE line for histogram family, got %d\n%s", count, body)
	}
}

func TestMetrics_BrowserGetsPlainText(t *testing.T) {
	resetStore(100)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rr := httptest.NewRecorder()
	handleMetrics(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain for browser request, got %q", ct)
	}
}

func TestMetrics_PrometheusGetsOpenMetrics(t *testing.T) {
	resetStore(100)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept", "application/openmetrics-text")
	rr := httptest.NewRecorder()
	handleMetrics(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/openmetrics-text") {
		t.Errorf("expected openmetrics content-type for Prometheus, got %q", ct)
	}
}

// ── Label deduplication / fingerprinting ─────────────────────────────────────

func TestPush_DifferentLabels_DifferentSeries(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	push(t, h, `{"name":"m","value":1,"labels":{"env":"prod"}}`, cfg.APIKey)
	push(t, h, `{"name":"m","value":2,"labels":{"env":"staging"}}`, cfg.APIKey)

	metrics.mutex.RLock()
	count := len(metrics.entries)
	metrics.mutex.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 separate series, got %d", count)
	}
}

func TestPush_SameLabels_UpdatesValue(t *testing.T) {
	resetStore(100)
	cfg := testConfig()
	h := requireAPIKey(cfg, handlePush)
	push(t, h, `{"name":"m","value":1,"labels":{"env":"prod"}}`, cfg.APIKey)
	push(t, h, `{"name":"m","value":99,"labels":{"env":"prod"}}`, cfg.APIKey)

	metrics.mutex.RLock()
	count := len(metrics.entries)
	entry := metrics.entries[`m{env="prod"}`]
	metrics.mutex.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 series after update, got %d", count)
	}
	if entry.Value != 99 {
		t.Errorf("expected updated value 99, got %g", entry.Value)
	}
}

// ── Canonical series key ──────────────────────────────────────────────────────

func TestCanonicalSeriesKey_NoLabels(t *testing.T) {
	key := canonicalSeriesKey("cpu", nil)
	if key != "cpu" {
		t.Errorf("expected \"cpu\", got %q", key)
	}
}

func TestCanonicalSeriesKey_LabelsAreSorted(t *testing.T) {
	// Regardless of map iteration order, keys must be sorted in the output.
	key := canonicalSeriesKey("m", map[string]string{"z": "1", "a": "2", "m": "3"})
	if key != `m{a="2",m="3",z="1"}` {
		t.Errorf("unexpected key %q", key)
	}
}

func TestCanonicalSeriesKey_LabelValueEscaping(t *testing.T) {
	key := canonicalSeriesKey("m", map[string]string{"k": `val"ue`})
	if !strings.Contains(key, `\"`) {
		t.Errorf("expected escaped quote in key, got %q", key)
	}
}

// ── metricFamilyName ──────────────────────────────────────────────────────────

func TestMetricFamilyName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"req_duration_bucket", "req_duration"},
		{"req_duration_sum", "req_duration"},
		{"req_duration_count", "req_duration"},
		{"http_requests_total", "http_requests"},
		// _info is a recognised OpenMetrics suffix; build_info → build family.
		{"build_info", "build"},
		{"cpu_usage", "cpu_usage"},
		{"events_created", "events"},
	}
	for _, tc := range cases {
		got := metricFamilyName(tc.in)
		if got != tc.want {
			t.Errorf("metricFamilyName(%q): expected %q, got %q", tc.in, tc.want, got)
		}
	}
}

// ── clientIP ──────────────────────────────────────────────────────────────────

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if ip := clientIP(req); ip != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %q", ip)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "5.6.7.8")
	if ip := clientIP(req); ip != "5.6.7.8" {
		t.Errorf("expected 5.6.7.8, got %q", ip)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.10.11.12:54321"
	if ip := clientIP(req); ip != "9.10.11.12" {
		t.Errorf("expected 9.10.11.12, got %q", ip)
	}
}
