package config

import (
	"fmt"
	"os"
	"strconv"
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

	// Dry run mode - if enabled, no actual DNS changes will be made
	DryRun bool
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

	return &Config{
		CustomerNumber:    customerNumber,
		APIKey:            apiKey,
		APIPassword:       apiPassword,
		DockerFilterLabel: os.Getenv("DOCKER_FILTER_LABEL"),
		DefaultTTL:        defaultTTL,
		DryRun:            dryRun,
	}, nil
}
