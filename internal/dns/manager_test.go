package dns

import (
	"context"
	"testing"

	"github.com/alex289/docker-traefik-netcup-companion/internal/config"
	"github.com/alex289/docker-traefik-netcup-companion/internal/docker"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         false,
	}

	manager := NewManager(cfg)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	if manager.config != cfg {
		t.Error("Manager config not set correctly")
	}

	if manager.client == nil {
		t.Error("Manager client not initialized")
	}

	if manager.knownHosts == nil {
		t.Error("Manager knownHosts map not initialized")
	}
}

func TestProcessHostInfo_DryRun(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         true, // Enable dry run mode
	}

	manager := NewManager(cfg)
	ctx := context.Background()

	info := docker.HostInfo{
		ContainerID:   "test123",
		ContainerName: "test-container",
		Hostname:      "app.example.com",
		Domain:        "example.com",
		Subdomain:     "app",
	}

	// In dry run mode with invalid credentials, it will try to login and fail
	// This is expected behavior - dry run now checks if record exists before deciding create vs update
	err := manager.ProcessHostInfo(ctx, info)
	if err == nil {
		t.Error("ProcessHostInfo() with invalid credentials should fail even in dry run mode")
	}

	// The error should be about login failure
	if err != nil && !contains(err.Error(), "failed to login") {
		t.Errorf("Expected login failure error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}()))
}

func TestProcessHostInfo_DuplicateHost(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         false, // Disable dry run to test duplicate logic
	}

	manager := NewManager(cfg)

	// Manually add host to knownHosts
	info := docker.HostInfo{
		ContainerID:   "test123",
		ContainerName: "test-container",
		Hostname:      "app.example.com",
		Domain:        "example.com",
		Subdomain:     "app",
	}

	manager.knownHosts[info.Hostname] = true

	ctx := context.Background()

	// Process same host - should be skipped
	err := manager.ProcessHostInfo(ctx, info)
	if err != nil {
		t.Errorf("ProcessHostInfo() on known host error = %v, want nil", err)
	}

	// Should still be in knownHosts
	if !manager.knownHosts[info.Hostname] {
		t.Error("Host removed from knownHosts after duplicate call")
	}
}

func TestProcessHostInfo_MultipleHosts(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         false, // Disable dry run
	}

	manager := NewManager(cfg)

	hosts := []docker.HostInfo{
		{
			ContainerID:   "test1",
			ContainerName: "container1",
			Hostname:      "app1.example.com",
			Domain:        "example.com",
			Subdomain:     "app1",
		},
		{
			ContainerID:   "test2",
			ContainerName: "container2",
			Hostname:      "app2.example.com",
			Domain:        "example.com",
			Subdomain:     "app2",
		},
		{
			ContainerID:   "test3",
			ContainerName: "container3",
			Hostname:      "api.example.com",
			Domain:        "example.com",
			Subdomain:     "api",
		},
	}

	// Manually add hosts to knownHosts to test the map functionality
	for _, info := range hosts {
		manager.knownHosts[info.Hostname] = true
	}

	// Verify all hosts are in knownHosts
	if len(manager.knownHosts) != len(hosts) {
		t.Errorf("knownHosts count = %d, want %d", len(manager.knownHosts), len(hosts))
	}

	for _, info := range hosts {
		if !manager.knownHosts[info.Hostname] {
			t.Errorf("Host %s not found in knownHosts", info.Hostname)
		}
	}
}

func TestGetHostIP(t *testing.T) {
	// This test verifies that getHostIP returns a valid IP address
	// Note: This test depends on network connectivity
	ip, err := getHostIP()
	if err != nil {
		t.Skipf("Skipping test - no network connectivity: %v", err)
	}

	if ip == "" {
		t.Error("getHostIP() returned empty string")
	}

	// Basic validation that it looks like an IP address
	// It should contain dots for IPv4
	if len(ip) < 7 { // minimum IPv4 is 0.0.0.0 (7 chars)
		t.Errorf("getHostIP() = %v, doesn't look like a valid IP", ip)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         false, // Disable dry run
	}

	manager := NewManager(cfg)

	// Pre-populate knownHosts to avoid API calls
	manager.knownHosts["app.example.com"] = true

	ctx := context.Background()

	// Test concurrent access to ProcessHostInfo
	done := make(chan bool)
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			info := docker.HostInfo{
				ContainerID:   "test",
				ContainerName: "container",
				Hostname:      "app.example.com",
				Domain:        "example.com",
				Subdomain:     "app",
			}
			_ = manager.ProcessHostInfo(ctx, info)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify the host is in knownHosts (should only be added once)
	if !manager.knownHosts["app.example.com"] {
		t.Error("Host not found in knownHosts after concurrent access")
	}
}

func TestManager_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         false, // Disable dry run
	}

	manager := NewManager(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	info := docker.HostInfo{
		ContainerID:   "test123",
		ContainerName: "test-container",
		Hostname:      "app.example.com",
		Domain:        "example.com",
		Subdomain:     "app",
	}

	// With cancelled context and invalid credentials, this should fail
	err := manager.ProcessHostInfo(ctx, info)
	if err == nil {
		t.Error("ProcessHostInfo() with cancelled context and invalid credentials should fail")
	}
	t.Logf("ProcessHostInfo() with cancelled context returned error (expected): %v", err)
}
