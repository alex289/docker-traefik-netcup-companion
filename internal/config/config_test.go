package config

import (
	"os"
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
