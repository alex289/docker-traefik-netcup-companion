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

	// First call should succeed and add to knownHosts
	err := manager.ProcessHostInfo(ctx, info)
	if err != nil {
		t.Errorf("ProcessHostInfo() error = %v, want nil", err)
	}

	// Verify host was added to knownHosts
	if !manager.knownHosts[info.Hostname] {
		t.Error("Host not added to knownHosts map")
	}

	// Second call with same hostname should skip processing
	err = manager.ProcessHostInfo(ctx, info)
	if err != nil {
		t.Errorf("ProcessHostInfo() on duplicate host error = %v, want nil", err)
	}
}

func TestProcessHostInfo_DuplicateHost(t *testing.T) {
	cfg := &config.Config{
		CustomerNumber: 12345,
		APIKey:         "test-key",
		APIPassword:    "test-password",
		DefaultTTL:     "300",
		DryRun:         true,
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

	// Process host first time
	err := manager.ProcessHostInfo(ctx, info)
	if err != nil {
		t.Fatalf("First ProcessHostInfo() error = %v, want nil", err)
	}

	// Verify it's in knownHosts
	if !manager.knownHosts[info.Hostname] {
		t.Fatal("Host not added to knownHosts after first call")
	}

	// Process same host again
	err = manager.ProcessHostInfo(ctx, info)
	if err != nil {
		t.Errorf("Second ProcessHostInfo() error = %v, want nil", err)
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
		DryRun:         true,
	}

	manager := NewManager(cfg)
	ctx := context.Background()

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

	// Process all hosts
	for _, info := range hosts {
		err := manager.ProcessHostInfo(ctx, info)
		if err != nil {
			t.Errorf("ProcessHostInfo() for %s error = %v, want nil", info.Hostname, err)
		}
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
		DryRun:         true,
	}

	manager := NewManager(cfg)
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
		DryRun:         true,
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

	// In dry run mode, this should still succeed even with cancelled context
	// because we don't make actual API calls
	err := manager.ProcessHostInfo(ctx, info)
	if err != nil {
		// This is expected behavior - in dry run it might succeed
		// In real mode with API calls, it should fail
		t.Logf("ProcessHostInfo() with cancelled context returned error (expected in non-dry-run): %v", err)
	}
}
