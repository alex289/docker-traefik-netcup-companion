package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/alex289/docker-traefik-netcup-companion/internal/config"
	"github.com/alex289/docker-traefik-netcup-companion/internal/docker"
	netcup "github.com/alex289/docker-traefik-netcup-companion/internal/netcup"
)

type Manager struct {
	config     *config.Config
	client     *netcup.NetcupDnsClient
	mu         sync.Mutex
	knownHosts map[string]bool // Track hosts we've already processed
}

func NewManager(cfg *config.Config) *Manager {
	client := netcup.NewNetcupDnsClient(cfg.CustomerNumber, cfg.APIKey, cfg.APIPassword)

	return &Manager{
		config:     cfg,
		client:     client,
		knownHosts: make(map[string]bool),
	}
}

func (m *Manager) ProcessHostInfo(ctx context.Context, info docker.HostInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we've already processed this host
	if m.knownHosts[info.Hostname] {
		log.Printf("Host %s already processed, skipping", info.Hostname)
		return nil
	}

	// Get the host's IP address
	var hostIP string
	if m.config.HostIP != "" {
		// Use configured IP
		hostIP = m.config.HostIP
		log.Printf("Using configured HOST_IP: %s", hostIP)
	} else {
		// Auto-detect IP
		var err error
		hostIP, err = getHostIP()
		if err != nil {
			return fmt.Errorf("failed to get host IP: %w", err)
		}
	}

	log.Printf("Processing DNS for %s -> %s", info.Hostname, hostIP)

	// Login to Netcup
	session, err := m.client.Login()
	if err != nil {
		return fmt.Errorf("failed to login to Netcup: %w", err)
	}
	defer session.Logout()

	// Check if DNS zone exists
	_, err = session.InfoDnsZone(info.Domain)
	if err != nil {
		return fmt.Errorf("failed to get DNS zone for %s: %w", info.Domain, err)
	}

	// Get existing DNS records
	records, err := session.InfoDnsRecords(info.Domain)
	if err != nil {
		return fmt.Errorf("failed to get DNS records for %s: %w", info.Domain, err)
	}

	// Check if record already exists
	recordExists := false
	var existingIP string
	for _, record := range *records {
		if record.Hostname == info.Subdomain && record.Type == "A" {
			existingIP = record.Destination
			if record.Destination == hostIP {
				log.Printf("DNS record for %s already exists with correct IP", info.Hostname)
				m.knownHosts[info.Hostname] = true
				return nil
			}
			recordExists = true
			log.Printf("DNS record for %s exists but with different IP (%s), will update", info.Hostname, record.Destination)
			break
		}
	}

	if m.config.DryRun {
		if recordExists {
			log.Printf("[DRY RUN] Would update DNS record: %s.%s (%s -> %s)", info.Subdomain, info.Domain, existingIP, hostIP)
		} else {
			log.Printf("[DRY RUN] Would create DNS record: %s.%s -> %s", info.Subdomain, info.Domain, hostIP)
		}
		m.knownHosts[info.Hostname] = true
		return nil
	}

	// Create or update the DNS record
	newRecord := netcup.DnsRecord{
		Hostname:    info.Subdomain,
		Type:        "A",
		Destination: hostIP,
		Priority:    "0",
	}

	if recordExists {
		log.Printf("Updating DNS record: %s.%s -> %s", info.Subdomain, info.Domain, hostIP)
	} else {
		log.Printf("Creating DNS record: %s.%s -> %s", info.Subdomain, info.Domain, hostIP)
	}

	recordSet := []netcup.DnsRecord{newRecord}
	_, err = session.UpdateDnsRecords(info.Domain, &recordSet)
	if err != nil {
		return fmt.Errorf("failed to update DNS records: %w", err)
	}

	m.knownHosts[info.Hostname] = true
	log.Printf("Successfully configured DNS for %s", info.Hostname)

	return nil
}

func getHostIP() (string, error) {
	// Try to get the default outbound IP
	// Note: This will return the local network IP, which may be private
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.String()

	// Check if this is a private IP
	if isPrivateIP(localAddr.IP) {
		log.Printf("Warning: Detected private IP %s. For DNS records, you should set HOST_IP environment variable to your public IP", ip)
	}

	return ip, nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}

	// Check for private IPv4 addresses
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}

	// Check for private IPv6 addresses
	if ip.To16() != nil {
		return len(ip) == net.IPv6len && ip[0] == 0xfc || ip[0] == 0xfd
	}

	return false
}
