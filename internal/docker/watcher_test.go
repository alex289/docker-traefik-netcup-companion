package docker

import (
	"testing"
)

func TestSplitHostname(t *testing.T) {
	tests := []struct {
		name          string
		hostname      string
		wantDomain    string
		wantSubdomain string
	}{
		{
			name:          "simple domain",
			hostname:      "example.com",
			wantDomain:    "example.com",
			wantSubdomain: "@",
		},
		{
			name:          "subdomain",
			hostname:      "app.example.com",
			wantDomain:    "example.com",
			wantSubdomain: "app",
		},
		{
			name:          "nested subdomain",
			hostname:      "api.app.example.com",
			wantDomain:    "example.com",
			wantSubdomain: "api.app",
		},
		{
			name:          "deep nested subdomain",
			hostname:      "v1.api.app.example.com",
			wantDomain:    "example.com",
			wantSubdomain: "v1.api.app",
		},
		{
			name:          "single part hostname",
			hostname:      "localhost",
			wantDomain:    "localhost",
			wantSubdomain: "@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDomain, gotSubdomain := splitHostname(tt.hostname)
			if gotDomain != tt.wantDomain {
				t.Errorf("splitHostname() domain = %v, want %v", gotDomain, tt.wantDomain)
			}
			if gotSubdomain != tt.wantSubdomain {
				t.Errorf("splitHostname() subdomain = %v, want %v", gotSubdomain, tt.wantSubdomain)
			}
		})
	}
}

func TestExtractHostsFromLabels(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		containerName string
		labels        map[string]string
		wantHosts     int
		checkHost     *HostInfo // Optional: specific host to verify
	}{
		{
			name:          "single host label",
			containerID:   "abc123",
			containerName: "/test-container",
			labels: map[string]string{
				"traefik.http.routers.myapp.rule": "Host(`app.example.com`)",
			},
			wantHosts: 1,
			checkHost: &HostInfo{
				ContainerID:   "abc123",
				ContainerName: "test-container",
				Hostname:      "app.example.com",
				Domain:        "example.com",
				Subdomain:     "app",
			},
		},
		{
			name:          "multiple routers with different hosts",
			containerID:   "def456",
			containerName: "/multi-container",
			labels: map[string]string{
				"traefik.http.routers.web.rule": "Host(`web.example.com`)",
				"traefik.http.routers.api.rule": "Host(`api.example.com`)",
			},
			wantHosts: 2,
		},
		{
			name:          "root domain",
			containerID:   "ghi789",
			containerName: "/root-container",
			labels: map[string]string{
				"traefik.http.routers.main.rule": "Host(`example.com`)",
			},
			wantHosts: 1,
			checkHost: &HostInfo{
				ContainerID:   "ghi789",
				ContainerName: "root-container",
				Hostname:      "example.com",
				Domain:        "example.com",
				Subdomain:     "@",
			},
		},
		{
			name:          "no traefik labels",
			containerID:   "jkl012",
			containerName: "/no-traefik",
			labels: map[string]string{
				"com.example.version": "1.0",
			},
			wantHosts: 0,
		},
		{
			name:          "complex rule with multiple hosts",
			containerID:   "mno345",
			containerName: "/complex-container",
			labels: map[string]string{
				"traefik.http.routers.multi.rule": "Host(`app1.example.com`) || Host(`app2.example.com`)",
			},
			wantHosts: 2,
		},
		{
			name:          "label without Host rule",
			containerID:   "pqr678",
			containerName: "/path-container",
			labels: map[string]string{
				"traefik.http.routers.path.rule": "PathPrefix(`/api`)",
			},
			wantHosts: 0,
		},
		{
			name:          "mixed rules",
			containerID:   "stu901",
			containerName: "/mixed-container",
			labels: map[string]string{
				"traefik.http.routers.main.rule": "Host(`example.com`) && PathPrefix(`/api`)",
			},
			wantHosts: 1,
			checkHost: &HostInfo{
				ContainerID:   "stu901",
				ContainerName: "mixed-container",
				Hostname:      "example.com",
				Domain:        "example.com",
				Subdomain:     "@",
			},
		},
		{
			name:          "deep nested subdomain",
			containerID:   "vwx234",
			containerName: "/nested-container",
			labels: map[string]string{
				"traefik.http.routers.api.rule": "Host(`v1.api.app.example.com`)",
			},
			wantHosts: 1,
			checkHost: &HostInfo{
				ContainerID:   "vwx234",
				ContainerName: "nested-container",
				Hostname:      "v1.api.app.example.com",
				Domain:        "example.com",
				Subdomain:     "v1.api.app",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHosts := extractHostsFromLabels(tt.containerID, tt.containerName, tt.labels)
			if len(gotHosts) != tt.wantHosts {
				t.Errorf("extractHostsFromLabels() returned %d hosts, want %d", len(gotHosts), tt.wantHosts)
				return
			}

			if tt.checkHost != nil && len(gotHosts) > 0 {
				found := false
				for _, host := range gotHosts {
					if host.Hostname == tt.checkHost.Hostname {
						found = true
						if host.ContainerID != tt.checkHost.ContainerID {
							t.Errorf("ContainerID = %v, want %v", host.ContainerID, tt.checkHost.ContainerID)
						}
						if host.ContainerName != tt.checkHost.ContainerName {
							t.Errorf("ContainerName = %v, want %v", host.ContainerName, tt.checkHost.ContainerName)
						}
						if host.Domain != tt.checkHost.Domain {
							t.Errorf("Domain = %v, want %v", host.Domain, tt.checkHost.Domain)
						}
						if host.Subdomain != tt.checkHost.Subdomain {
							t.Errorf("Subdomain = %v, want %v", host.Subdomain, tt.checkHost.Subdomain)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected host %v not found in results", tt.checkHost.Hostname)
				}
			}
		})
	}
}

func TestExtractHostsFromLabels_ContainerNameTrimming(t *testing.T) {
	labels := map[string]string{
		"traefik.http.routers.test.rule": "Host(`test.example.com`)",
	}

	// Test with leading slash
	hosts := extractHostsFromLabels("container123", "/my-container", labels)
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}
	if hosts[0].ContainerName != "my-container" {
		t.Errorf("ContainerName = %v, want 'my-container' (without leading slash)", hosts[0].ContainerName)
	}

	// Test without leading slash
	hosts = extractHostsFromLabels("container456", "another-container", labels)
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}
	if hosts[0].ContainerName != "another-container" {
		t.Errorf("ContainerName = %v, want 'another-container'", hosts[0].ContainerName)
	}
}

func TestHostInfo(t *testing.T) {
	// Test HostInfo struct creation
	info := HostInfo{
		ContainerID:   "test123",
		ContainerName: "test-container",
		Hostname:      "app.example.com",
		Domain:        "example.com",
		Subdomain:     "app",
	}

	if info.ContainerID != "test123" {
		t.Errorf("ContainerID = %v, want test123", info.ContainerID)
	}
	if info.ContainerName != "test-container" {
		t.Errorf("ContainerName = %v, want test-container", info.ContainerName)
	}
	if info.Hostname != "app.example.com" {
		t.Errorf("Hostname = %v, want app.example.com", info.Hostname)
	}
	if info.Domain != "example.com" {
		t.Errorf("Domain = %v, want example.com", info.Domain)
	}
	if info.Subdomain != "app" {
		t.Errorf("Subdomain = %v, want app", info.Subdomain)
	}
}
