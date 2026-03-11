// HTTP handler functions
package main

import (
	_ "embed"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

//go:embed index.html
var indexHTML []byte

func registerHandlers(config *Config) {
	http.Handle("/", logRequest(http.HandlerFunc(handleIndex)))
	http.Handle("/health", logRequest(http.HandlerFunc(handleHealth)))
	http.Handle("/metrics", logRequest(http.HandlerFunc(handleMetrics)))
	http.Handle("/push", logRequest(requireAPIKey(config, handlePush)))
}

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// logRequest is middleware that logs the method, path, status code, duration,
// and client IP of every request.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %s %d %s", clientIP(r), r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

// clientIP extracts the real client IP, respecting X-Forwarded-For and
// X-Real-IP headers set by a reverse proxy in front of Tally.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be a comma-separated list; the first entry is the client.
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to the remote address, stripping the port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// requireAPIKey is middleware that validates the API key from the
// Authorization header before passing the request to the next handler.
func requireAPIKey(config *Config, next http.HandlerFunc) http.HandlerFunc {
	expected := []byte("Bearer " + config.APIKey)
	return func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// The default mux routes any unmatched path to "/", so explicitly 404
	// anything that isn't exactly the root.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleMetrics renders all stored metrics in OpenMetrics text format, which Prometheus can scrape.
// Series are grouped by their metric family so that a single # TYPE line is emitted per family.
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	metrics.mutex.RLock()
	snap := make(map[string]MetricEntry, len(metrics.entries))
	for k, e := range metrics.entries {
		snap[k] = e
	}
	metrics.mutex.RUnlock()

	// Sort series keys for deterministic output.
	keys := make([]string, 0, len(snap))
	for k := range snap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Group series by their metric family name so we emit one # TYPE per family.
	type familyGroup struct {
		metricType string
		members    []string
	}
	groupOrder := make([]string, 0)
	families := make(map[string]*familyGroup)
	for _, key := range keys {
		family := metricFamilyName(snap[key].Name)
		if _, ok := families[family]; !ok {
			groupOrder = append(groupOrder, family)
			families[family] = &familyGroup{metricType: snap[key].Type}
		}
		families[family].members = append(families[family].members, key)
	}

	// Browsers send Accept: text/html — serve plain text so the page renders
	// instead of downloading. Prometheus scrapers explicitly request
	// application/openmetrics-text and will still get the correct type.
	contentType := "application/openmetrics-text; version=1.0.0; charset=utf-8"
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		contentType = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)

	// Always emit tally_up 1 first so there is always at least one metric to
	// scrape and dashboards have a reliable signal that Tally is running.
	fmt.Fprintln(w, "# TYPE tally_up gauge")
	fmt.Fprintln(w, "tally_up 1")

	for _, family := range groupOrder {
		fg := families[family]
		fmt.Fprintf(w, "# TYPE %s %s\n", family, fg.metricType)
		for _, key := range fg.members {
			fmt.Fprintf(w, "%s %g\n", key, snap[key].Value)
		}
	}
	// OpenMetrics requires an explicit EOF marker.
	fmt.Fprintln(w, "# EOF")
}

func handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	// Cap the request body at 64 KB to prevent memory exhaustion.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var payload struct {
		Name   string            `json:"name"`
		Value  float64           `json:"value"`
		Type   string            `json:"type"`   // optional: gauge (default), counter, histogram, summary, untyped
		Labels map[string]string `json:"labels"` // optional: {"instance": "web-1", "job": "app"}
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !validateMetricName(payload.Name) {
		http.Error(w, "Invalid metric name: must match [a-zA-Z_][a-zA-Z0-9_]* and be 1-200 chars", http.StatusBadRequest)
		return
	}
	if !validateMetricValue(payload.Value) {
		http.Error(w, "Invalid metric value: must be a finite number", http.StatusBadRequest)
		return
	}
	if !validateMetricType(payload.Type) {
		http.Error(w, "Invalid metric type: must be one of gauge, counter, histogram, summary, untyped", http.StatusBadRequest)
		return
	}
	if !validateLabels(payload.Labels) {
		http.Error(w, "Invalid labels: names must match [a-zA-Z_][a-zA-Z0-9_]*, must not start with __, values must not contain null bytes", http.StatusBadRequest)
		return
	}

	metricType := payload.Type
	if metricType == "" {
		metricType = "gauge"
	}

	seriesKey := canonicalSeriesKey(payload.Name, payload.Labels)

	metrics.mutex.Lock()
	_, exists := metrics.entries[seriesKey]
	if !exists && len(metrics.entries) >= metrics.maxMetrics {
		metrics.mutex.Unlock()
		http.Error(w, "Metric limit reached", http.StatusTooManyRequests)
		return
	}
	metrics.entries[seriesKey] = MetricEntry{
		Name:    payload.Name,
		Value:   payload.Value,
		Type:    metricType,
		Labels:  payload.Labels,
		LastUpdatedAt: time.Now(),
	}
	metrics.mutex.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Series %s (%s) updated to %g", seriesKey, metricType, payload.Value)
}
