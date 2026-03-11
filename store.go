// MetricStore: types, global instance, initialization, and TTL expiry.
package main

import (
	"log"
	"sync"
	"time"
)

// MetricEntry holds a single time series: its value, OpenMetrics type,
// the label set it was pushed with, and when it was last written.
type MetricEntry struct {
	Name    string
	Value   float64
	Type    string
	Labels  map[string]string
	LastUpdatedAt time.Time
}

// MetricStore holds all active series keyed by their canonical series key,
// e.g. cpu_usage{instance="web-1",job="tally"}.
type MetricStore struct {
	entries    map[string]MetricEntry
	maxMetrics int
	mutex      sync.RWMutex
}

var metrics *MetricStore

func initMetrics(config *Config) {
	metrics = &MetricStore{
		entries:    make(map[string]MetricEntry),
		maxMetrics: config.MaxMetrics,
	}
	go startMetricExpiry(config.MetricTTL)
}

// startMetricExpiry runs a background goroutine that removes series from the
// MetricStore that were last written more than ttlMinutes minutes ago.
func startMetricExpiry(ttlMinutes int) {
	ttl := time.Duration(ttlMinutes) * time.Minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		metrics.mutex.Lock()
		for key, entry := range metrics.entries {
			if now.Sub(entry.LastUpdatedAt) > ttl {
				delete(metrics.entries, key)
				log.Printf("Expired metric: %s", key)
			}
		}
		metrics.mutex.Unlock()
	}
}
