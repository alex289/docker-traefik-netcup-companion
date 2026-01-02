package config

import (
	"os"
	"strconv"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid configuration",
			envVars: map[string]string{
				"NC_CUSTOMER_NUMBER": "12345",
				"NC_API_KEY":         "test-api-key",
				"NC_API_PASSWORD":    "test-api-password",
			},
			wantErr: false,
		},
		{
			name: "valid configuration with optional values",
			envVars: map[string]string{
				"NC_CUSTOMER_NUMBER":  "12345",
				"NC_API_KEY":          "test-api-key",
				"NC_API_PASSWORD":     "test-api-password",
				"NC_DEFAULT_TTL":      "600",
				"DOCKER_FILTER_LABEL": "traefik.enable=true",
				"DRY_RUN":             "true",
				"NOTIFICATION_URLS":   "slack://token@channel,discord://token@id",
			},
			wantErr: false,
		},
		{
			name: "missing customer number",
			envVars: map[string]string{
				"NC_API_KEY":      "test-api-key",
				"NC_API_PASSWORD": "test-api-password",
			},
			wantErr: true,
		},
		{
			name: "invalid customer number",
			envVars: map[string]string{
				"NC_CUSTOMER_NUMBER": "not-a-number",
				"NC_API_KEY":         "test-api-key",
				"NC_API_PASSWORD":    "test-api-password",
			},
			wantErr: true,
		},
		{
			name: "missing API key",
			envVars: map[string]string{
				"NC_CUSTOMER_NUMBER": "12345",
				"NC_API_PASSWORD":    "test-api-password",
			},
			wantErr: true,
		},
		{
			name: "missing API password",
			envVars: map[string]string{
				"NC_CUSTOMER_NUMBER": "12345",
				"NC_API_KEY":         "test-api-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables
			os.Clearenv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify configuration values
			if tt.envVars["NC_CUSTOMER_NUMBER"] != "" {
				if cfg.CustomerNumber == 0 {
					t.Errorf("CustomerNumber not set")
				}
			}

			if tt.envVars["NC_API_KEY"] != "" {
				if cfg.APIKey != tt.envVars["NC_API_KEY"] {
					t.Errorf("APIKey = %v, want %v", cfg.APIKey, tt.envVars["NC_API_KEY"])
				}
			}

			if tt.envVars["NC_API_PASSWORD"] != "" {
				if cfg.APIPassword != tt.envVars["NC_API_PASSWORD"] {
					t.Errorf("APIPassword = %v, want %v", cfg.APIPassword, tt.envVars["NC_API_PASSWORD"])
				}
			}

			// Test default TTL
			expectedTTL := "300"
			if val, ok := tt.envVars["NC_DEFAULT_TTL"]; ok {
				expectedTTL = val
			}
			if cfg.DefaultTTL != expectedTTL {
				t.Errorf("DefaultTTL = %v, want %v", cfg.DefaultTTL, expectedTTL)
			}

			// Test dry run
			expectedDryRun := tt.envVars["DRY_RUN"] == "true" || tt.envVars["DRY_RUN"] == "1"
			if cfg.DryRun != expectedDryRun {
				t.Errorf("DryRun = %v, want %v", cfg.DryRun, expectedDryRun)
			}

			// Test Docker filter label
			if val, ok := tt.envVars["DOCKER_FILTER_LABEL"]; ok {
				if cfg.DockerFilterLabel != val {
					t.Errorf("DockerFilterLabel = %v, want %v", cfg.DockerFilterLabel, val)
				}
			}

			// Test notification URLs
			if val, ok := tt.envVars["NOTIFICATION_URLS"]; ok {
				if val == "" && len(cfg.NotificationURLs) != 0 {
					t.Errorf("NotificationURLs = %v, want empty slice", cfg.NotificationURLs)
				}
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("NC_CUSTOMER_NUMBER", "12345")
	os.Setenv("NC_API_KEY", "test-key")
	os.Setenv("NC_API_PASSWORD", "test-password")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test default TTL
	if cfg.DefaultTTL != "300" {
		t.Errorf("DefaultTTL = %v, want 300", cfg.DefaultTTL)
	}

	// Test default DryRun
	if cfg.DryRun != false {
		t.Errorf("DryRun = %v, want false", cfg.DryRun)
	}

	// Test default DockerFilterLabel
	if cfg.DockerFilterLabel != "" {
		t.Errorf("DockerFilterLabel = %v, want empty string", cfg.DockerFilterLabel)
	}

	// Test default NotificationURLs
	if len(cfg.NotificationURLs) != 0 {
		t.Errorf("NotificationURLs = %v, want empty slice", cfg.NotificationURLs)
	}
}

func TestLoadDryRunVariations(t *testing.T) {
	testCases := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
		{"yes", false}, // Should be false as only "true" or "1" enable it
	}

	for _, tc := range testCases {
		t.Run("DRY_RUN="+tc.value, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("NC_CUSTOMER_NUMBER", "12345")
			os.Setenv("NC_API_KEY", "test-key")
			os.Setenv("NC_API_PASSWORD", "test-password")
			os.Setenv("DRY_RUN", tc.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.DryRun != tc.expected {
				t.Errorf("DryRun = %v, want %v for DRY_RUN=%v", cfg.DryRun, tc.expected, tc.value)
			}
		})
	}
}
func TestLoadNotificationURLs(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		expected []string
	}{
		{
			name:     "no URLs",
			value:    "",
			expected: []string{},
		},
		{
			name:     "single URL",
			value:    "slack://token@channel",
			expected: []string{"slack://token@channel"},
		},
		{
			name:     "multiple URLs",
			value:    "slack://token@channel,discord://token@id",
			expected: []string{"slack://token@channel", "discord://token@id"},
		},
		{
			name:     "URLs with spaces",
			value:    "slack://token@channel, discord://token@id , generic://webhook",
			expected: []string{"slack://token@channel", "discord://token@id", "generic://webhook"},
		},
		{
			name:     "URLs with trailing comma",
			value:    "slack://token@channel,",
			expected: []string{"slack://token@channel"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("NC_CUSTOMER_NUMBER", "12345")
			os.Setenv("NC_API_KEY", "test-key")
			os.Setenv("NC_API_PASSWORD", "test-password")
			os.Setenv("NOTIFICATION_URLS", tc.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if len(cfg.NotificationURLs) != len(tc.expected) {
				t.Errorf("NotificationURLs length = %v, want %v", len(cfg.NotificationURLs), len(tc.expected))
			}

			for i, url := range tc.expected {
				if i >= len(cfg.NotificationURLs) {
					t.Errorf("Missing URL at index %v, want %v", i, url)
					continue
				}
				if cfg.NotificationURLs[i] != url {
					t.Errorf("NotificationURLs[%v] = %v, want %v", i, cfg.NotificationURLs[i], url)
				}
			}
		})
	}
}

// FuzzLoad tests the config.Load function with random input data
// Run with: go test -fuzz=FuzzLoad -fuzztime=30s
func FuzzLoad(f *testing.F) {
	// Add seed corpus for common scenarios
	f.Add("12345", "api-key", "api-password", "300", "true", "slack://token@channel", "traefik.enable=true", "192.168.1.1")
	f.Add("999999", "key123", "pass123", "600", "false", "", "", "")
	f.Add("0", "", "", "", "", "", "", "")
	f.Add("-1", "special!@#$%^&*()", "pass\nwith\nnewlines", "3600", "1", "invalid,url,format", "label", "10.0.0.1")
	f.Add("abc", "key", "pass", "invalid", "yes", "http://example.com,https://test.com", "", "")

	f.Fuzz(func(t *testing.T, customerNumber, apiKey, apiPassword, defaultTTL, dryRun, notificationURLs, dockerFilterLabel, hostIP string) {
		// Clear environment and set fuzzed values
		os.Clearenv()
		os.Setenv("NC_CUSTOMER_NUMBER", customerNumber)
		os.Setenv("NC_API_KEY", apiKey)
		os.Setenv("NC_API_PASSWORD", apiPassword)
		os.Setenv("NC_DEFAULT_TTL", defaultTTL)
		os.Setenv("DRY_RUN", dryRun)
		os.Setenv("NOTIFICATION_URLS", notificationURLs)
		os.Setenv("DOCKER_FILTER_LABEL", dockerFilterLabel)
		os.Setenv("HOST_IP", hostIP)

		cfg, err := Load()

		// Verify that the function doesn't panic and returns consistent results
		if err != nil {
			// If there's an error, config should be nil
			if cfg != nil {
				t.Errorf("Load() returned error but config is not nil: %v", err)
			}

			// Basic error checks - error message should not be empty
			if err.Error() == "" {
				t.Errorf("Load() returned empty error message")
			}
			return
		}

		// If no error, config must not be nil
		if cfg == nil {
			t.Fatal("Load() returned nil config without error")
		}

		// Verify customerNumber is valid integer when parsed successfully
		if customerNumberInt, parseErr := strconv.Atoi(customerNumber); parseErr == nil {
			if cfg.CustomerNumber != customerNumberInt {
				t.Errorf("CustomerNumber = %v, want %v", cfg.CustomerNumber, customerNumberInt)
			}
		}

		// Verify required fields are set
		if cfg.APIKey == "" {
			t.Error("APIKey should not be empty when Load() succeeds")
		}
		if cfg.APIPassword == "" {
			t.Error("APIPassword should not be empty when Load() succeeds")
		}

		// Verify default TTL is set
		if cfg.DefaultTTL == "" {
			t.Error("DefaultTTL should not be empty (should default to 300)")
		}

		// Verify DryRun is a valid boolean result
		// No panic means it's valid
		_ = cfg.DryRun

		// NotificationURLs can be nil or an empty/non-empty slice depending on input
		// Just verify it doesn't cause issues when accessed
		_ = cfg.NotificationURLs

		// Verify fields are set as expected from environment
		if cfg.APIKey != apiKey {
			t.Errorf("APIKey = %v, want %v", cfg.APIKey, apiKey)
		}
		if cfg.APIPassword != apiPassword {
			t.Errorf("APIPassword = %v, want %v", cfg.APIPassword, apiPassword)
		}
		// Note: DockerFilterLabel and HostIP may differ from input due to environment variable handling
		// (e.g., null bytes are not preserved in environment variables)
		// Just verify they don't cause panics
		_ = cfg.DockerFilterLabel
		_ = cfg.HostIP
	})
}
