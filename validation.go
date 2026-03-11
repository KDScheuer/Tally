// metric name validation, value validation, label and input sanitization.
package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Prometheus metric names must match [a-zA-Z_:][a-zA-Z0-9_:]*
// Colons are reserved for recording rules, so we disallow them for pushed metrics.
var validMetricName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateMetricName checks that the name conforms to Prometheus naming rules:
//   - starts with a letter or underscore
//   - contains only letters, digits, and underscores
//   - is between 1 and 200 characters long
func validateMetricName(name string) bool {
	if len(name) == 0 || len(name) > 200 {
		return false
	}
	return validMetricName.MatchString(name)
}

// validateMetricValue ensures the value is a finite float64.
// NaN and ±Inf are rejected to keep the store well-defined.
func validateMetricValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// validMetricTypes is the full set of valid Prometheus/OpenMetrics metric types.
// histogram and summary series must be pushed individually using the conventional
// suffixes (_bucket, _sum, _count, _created, _total) — each push is one series.
var validMetricTypes = map[string]bool{
	"gauge":     true,
	"counter":   true,
	"histogram": true,
	"summary":   true,
	"untyped":   true,
}

// validateMetricType returns true when t is a supported OpenMetrics type.
// An empty string is accepted and treated as "gauge" by the push handler.
func validateMetricType(t string) bool {
	if t == "" {
		return true
	}
	return validMetricTypes[t]
}

// validLabelName matches Prometheus label naming rules.
// Names beginning with "__" are reserved for internal use.
var validLabelName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateLabelName returns true when the label name is valid and not reserved.
func validateLabelName(name string) bool {
	if strings.HasPrefix(name, "__") {
		return false
	}
	return validLabelName.MatchString(name)
}

// validateLabelValue returns true when the label value contains no null bytes.
func validateLabelValue(v string) bool {
	return !strings.ContainsRune(v, '\x00')
}

// validateLabels checks every label name and value in the map.
func validateLabels(labels map[string]string) bool {
	for k, v := range labels {
		if !validateLabelName(k) || !validateLabelValue(v) {
			return false
		}
	}
	return true
}

// labelEscape escapes a label value for the OpenMetrics text format.
func labelEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// canonicalSeriesKey builds the unique key for a series in the form:
//
//	metric_name                              (no labels)
//	metric_name{instance="web-1",job="app"}  (with labels, sorted)
func canonicalSeriesKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(name)
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(k)
		sb.WriteString(`="`)
		sb.WriteString(labelEscape(labels[k]))
		sb.WriteByte('"')
	}
	sb.WriteByte('}')
	return sb.String()
}

// familySuffixes are the conventional Prometheus series suffixes that are
// stripped to derive the metric family name used in # TYPE declarations.
var familySuffixes = []string{
	"_created", // longest first to avoid partial stripping
	"_bucket",
	"_total",
	"_count",
	"_sum",
	"_info",
}

// metricFamilyName returns the Prometheus metric family name for a given
// series name by stripping any known suffix. For example:
//
//	request_duration_bucket → request_duration
//	http_requests_total     → http_requests
//	cpu_usage               → cpu_usage (unchanged)
func metricFamilyName(name string) string {
	for _, suffix := range familySuffixes {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}