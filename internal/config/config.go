package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Netcup credentials
	CustomerNumber int
	APIKey         string
	APIPassword    string

	// Docker filter label (optional)
	DockerFilterLabel string

	// Default TTL for DNS records (in seconds)
	DefaultTTL string

	// Host IP - if set, this IP will be used for DNS records instead of auto-detection
	HostIP string

	// Dry run mode - if enabled, no actual DNS changes will be made
	DryRun bool

	// Notification URLs - optional webhook URLs for notifications (shoutrrr format)
	NotificationURLs []string

	// Retry settings
	MaxRetries        int     // Maximum number of retry attempts (default: 3)
	InitialBackoff    int     // Initial backoff in milliseconds (default: 1000)
	MaxBackoff        int     // Maximum backoff in milliseconds (default: 30000)
	BackoffMultiplier float64 // Backoff multiplier (default: 2.0)

	// Circuit breaker settings
	CircuitBreakerThreshold    int // Number of consecutive failures to open circuit (default: 5)
	CircuitBreakerTimeout      int // Circuit breaker timeout in seconds (default: 60)
	CircuitBreakerHalfOpenReqs int // Number of requests to try in half-open state (default: 3)
}

func Load() (*Config, error) {
	customerNumberStr := os.Getenv("NC_CUSTOMER_NUMBER")
	if customerNumberStr == "" {
		return nil, fmt.Errorf("NC_CUSTOMER_NUMBER environment variable is required")
	}

	customerNumber, err := strconv.Atoi(customerNumberStr)
	if err != nil {
		return nil, fmt.Errorf("NC_CUSTOMER_NUMBER must be a valid integer: %w", err)
	}

	apiKey := os.Getenv("NC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("NC_API_KEY environment variable is required")
	}

	apiPassword := os.Getenv("NC_API_PASSWORD")
	if apiPassword == "" {
		return nil, fmt.Errorf("NC_API_PASSWORD environment variable is required")
	}

	defaultTTL := os.Getenv("NC_DEFAULT_TTL")
	if defaultTTL == "" {
		defaultTTL = "300" // 5 minutes default
	}

	dryRun := false
	if os.Getenv("DRY_RUN") == "true" || os.Getenv("DRY_RUN") == "1" {
		dryRun = true
	}

	// Parse retry settings with defaults
	maxRetries := getEnvAsInt("NC_MAX_RETRIES", 3)
	initialBackoff := getEnvAsInt("NC_INITIAL_BACKOFF_MS", 1000)
	maxBackoff := getEnvAsInt("NC_MAX_BACKOFF_MS", 30000)
	backoffMultiplier := getEnvAsFloat("NC_BACKOFF_MULTIPLIER", 2.0)

	// Parse circuit breaker settings with defaults
	circuitBreakerThreshold := getEnvAsInt("NC_CIRCUIT_BREAKER_THRESHOLD", 5)
	circuitBreakerTimeout := getEnvAsInt("NC_CIRCUIT_BREAKER_TIMEOUT_SEC", 60)
	circuitBreakerHalfOpenReqs := getEnvAsInt("NC_CIRCUIT_BREAKER_HALF_OPEN_REQS", 3)

	// Parse notification URLs (comma-separated)
	var notificationURLs []string
	if notificationURLsStr := os.Getenv("NOTIFICATION_URLS"); notificationURLsStr != "" {
		for _, url := range strings.Split(notificationURLsStr, ",") {
			if trimmed := strings.TrimSpace(url); trimmed != "" {
				notificationURLs = append(notificationURLs, trimmed)
			}
		}
	}

	return &Config{
		CustomerNumber:             customerNumber,
		APIKey:                     apiKey,
		APIPassword:                apiPassword,
		DockerFilterLabel:          os.Getenv("DOCKER_FILTER_LABEL"),
		DefaultTTL:                 defaultTTL,
		HostIP:                     os.Getenv("HOST_IP"),
		DryRun:                     dryRun,
		NotificationURLs:           notificationURLs,
		MaxRetries:                 maxRetries,
		InitialBackoff:             initialBackoff,
		MaxBackoff:                 maxBackoff,
		BackoffMultiplier:          backoffMultiplier,
		CircuitBreakerThreshold:    circuitBreakerThreshold,
		CircuitBreakerTimeout:      circuitBreakerTimeout,
		CircuitBreakerHalfOpenReqs: circuitBreakerHalfOpenReqs,
	}, nil
}

func getEnvAsInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	if val := os.Getenv(key); val != "" {
		if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}
