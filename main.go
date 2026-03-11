// starts the server, loads config from env, wires everything together.
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

type Config struct {
	APIKey      string
	Port        int
	MetricTTL   int
	MaxMetrics  int
	BindAddress string
}

func main() {
	log.Printf("Starting Tally server...")

	config := getConfig()
	log.Printf("Configuration: port=%d bind=%s ttl=%dm max_metrics=%d",
		config.Port, config.BindAddress, config.MetricTTL, config.MaxMetrics)

	initMetrics(config)
	if err := startHTTPServer(config); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getConfig() *Config {
	apiKey      := os.Getenv("API_KEY")
	port        := getEnvOrDefault("PORT", "9200")
	metricTTL   := getEnvOrDefault("METRIC_TTL", "1440")
	maxMetrics  := getEnvOrDefault("MAX_METRICS", "1000")
	bindAddress := getEnvOrDefault("BIND_ADDRESS", "0.0.0.0")

	if apiKey == ""{
		log.Fatal("API_KEY environment variable is required")
	}

	portInt, err := strconv.Atoi(port)
	if err != nil || portInt < 1 || portInt > 65535 {
		log.Fatal("PORT must be an integer between 1 and 65535")
	}

	ttlInt, err := strconv.Atoi(metricTTL)
	if err != nil || ttlInt <= 0 {
		log.Fatal("METRIC_TTL must be a positive integer")
	}

	maxInt, err := strconv.Atoi(maxMetrics)
	if err != nil || maxInt <= 0 {
		log.Fatal("MAX_METRICS must be a positive integer")
	}

	if net.ParseIP(bindAddress) == nil {
		log.Fatal("BIND_ADDRESS must be a valid IP address")
	}

	return &Config{
		APIKey:      apiKey,
		Port:        portInt,
		MetricTTL:   ttlInt,
		MaxMetrics:  maxInt,
		BindAddress: bindAddress,
	}
}

func startHTTPServer(config *Config) error {
	registerHandlers(config)
	addr := net.JoinHostPort(config.BindAddress, strconv.Itoa(config.Port))

	srv := &http.Server{Addr: addr}

	// Listen for SIGTERM (docker stop) or SIGINT (Ctrl+C) and drain in-flight
	// requests before exiting.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Printf("Shutting down...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	log.Printf("Listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}