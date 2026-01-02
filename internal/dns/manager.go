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
	"github.com/alex289/docker-traefik-netcup-companion/internal/notification"
	"github.com/alex289/docker-traefik-netcup-companion/internal/state"
)

type Manager struct {
	config       *config.Config
	client       *netcup.NetcupDnsClient
	notifier     *notification.Notifier
	stateManager *state.Manager
	mu           sync.Mutex
	knownHosts   map[string]bool // Track hosts we've already processed
}

func NewManager(cfg *config.Config, stateManager *state.Manager) *Manager {
	client := netcup.NewNetcupDnsClient(cfg.CustomerNumber, cfg.APIKey, cfg.APIPassword)
	notifier := notification.NewNotifier(cfg.NotificationURLs)

	return &Manager{
		config:       cfg,
		client:       client,
		notifier:     notifier,
		stateManager: stateManager,
		knownHosts:   make(map[string]bool),
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
		m.notifier.SendError(fmt.Sprintf("Failed to login to Netcup for %s: %v", info.Hostname, err))
		return fmt.Errorf("failed to login to Netcup: %w", err)
	}
	defer session.Logout()

	// Check if DNS zone exists
	_, err = session.InfoDnsZone(info.Domain)
	if err != nil {
		m.notifier.SendError(fmt.Sprintf("Failed to get DNS zone for %s: %v", info.Domain, err))
		return fmt.Errorf("failed to get DNS zone for %s: %w", info.Domain, err)
	}

	// Get existing DNS records
	records, err := session.InfoDnsRecords(info.Domain)
	if err != nil {
		m.notifier.SendError(fmt.Sprintf("Failed to get DNS records for %s: %v", info.Domain, err))
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
			m.notifier.SendInfo(fmt.Sprintf("[DRY RUN] Would update DNS: %s (%s -> %s)", info.Hostname, existingIP, hostIP))
		} else {
			log.Printf("[DRY RUN] Would create DNS record: %s.%s -> %s", info.Subdomain, info.Domain, hostIP)
			m.notifier.SendInfo(fmt.Sprintf("[DRY RUN] Would create DNS: %s -> %s", info.Hostname, hostIP))
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
		m.notifier.SendError(fmt.Sprintf("Failed to update DNS for %s: %v", info.Hostname, err))
		return fmt.Errorf("failed to update DNS records: %w", err)
	}

	m.knownHosts[info.Hostname] = true
	log.Printf("Successfully configured DNS for %s", info.Hostname)

	// Persist state to disk
	if m.stateManager != nil {
		if err := m.stateManager.UpdateRecord(info.Hostname, info.Domain, info.Subdomain, hostIP, "A"); err != nil {
			log.Printf("Warning: Failed to persist DNS state for %s: %v", info.Hostname, err)
		}
	}

	if recordExists {
		m.notifier.SendSuccess(fmt.Sprintf("Updated DNS: %s -> %s", info.Hostname, hostIP))
	} else {
		m.notifier.SendSuccess(fmt.Sprintf("Created DNS: %s -> %s", info.Hostname, hostIP))
	}

	return nil
}

// ReconcileFromState performs startup reconciliation by comparing persisted state
// with actual DNS records and syncing any drift
func (m *Manager) ReconcileFromState(ctx context.Context) error {
	if m.stateManager == nil || !m.stateManager.HasRecords() {
		log.Println("No persisted state to reconcile")
		return nil
	}

	records := m.stateManager.GetRecordsForReconciliation()
	log.Printf("Starting reconciliation for %d persisted DNS records", len(records))

	// Get the host's IP address
	var hostIP string
	if m.config.HostIP != "" {
		hostIP = m.config.HostIP
	} else {
		var err error
		hostIP, err = getHostIP()
		if err != nil {
			return fmt.Errorf("failed to get host IP for reconciliation: %w", err)
		}
	}

	// Login to Netcup
	session, err := m.client.Login()
	if err != nil {
		return fmt.Errorf("failed to login to Netcup for reconciliation: %w", err)
	}
	defer session.Logout()

	// Group records by domain to minimize API calls
	recordsByDomain := make(map[string][]state.DNSRecord)
	for _, record := range records {
		recordsByDomain[record.Domain] = append(recordsByDomain[record.Domain], record)
	}

	var syncedCount, skippedCount, errorCount int

	for domain, domainRecords := range recordsByDomain {
		// Get existing DNS records for this domain
		existingRecords, err := session.InfoDnsRecords(domain)
		if err != nil {
			log.Printf("Warning: Failed to get DNS records for %s during reconciliation: %v", domain, err)
			errorCount += len(domainRecords)
			continue
		}

		// Build a map of existing records
		existingMap := make(map[string]string) // subdomain -> IP
		for _, er := range *existingRecords {
			if er.Type == "A" {
				existingMap[er.Hostname] = er.Destination
			}
		}

		// Check each persisted record
		for _, record := range domainRecords {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			existingIP, exists := existingMap[record.Subdomain]

			// Determine expected IP (use current host IP, not persisted IP, to handle IP changes)
			expectedIP := hostIP

			if exists && existingIP == expectedIP {
				log.Printf("Reconciliation: %s is in sync (IP: %s)", record.Hostname, existingIP)
				skippedCount++
				m.knownHosts[record.Hostname] = true
				continue
			}

			if m.config.DryRun {
				if exists {
					log.Printf("[DRY RUN] Reconciliation would update: %s (%s -> %s)", record.Hostname, existingIP, expectedIP)
				} else {
					log.Printf("[DRY RUN] Reconciliation would create: %s -> %s", record.Hostname, expectedIP)
				}
				m.knownHosts[record.Hostname] = true
				skippedCount++
				continue
			}

			// Need to sync this record
			action := "create"
			if exists {
				action = "update"
			}

			log.Printf("Reconciliation: %s needs %s (%s -> %s)", record.Hostname, action, existingIP, expectedIP)

			newRecord := netcup.DnsRecord{
				Hostname:    record.Subdomain,
				Type:        "A",
				Destination: expectedIP,
				Priority:    "0",
			}

			recordSet := []netcup.DnsRecord{newRecord}
			_, err = session.UpdateDnsRecords(domain, &recordSet)
			if err != nil {
				log.Printf("Warning: Failed to reconcile DNS for %s: %v", record.Hostname, err)
				m.notifier.SendError(fmt.Sprintf("Reconciliation failed for %s: %v", record.Hostname, err))
				errorCount++
				continue
			}

			// Update persisted state with new IP
			if err := m.stateManager.UpdateRecord(record.Hostname, record.Domain, record.Subdomain, expectedIP, "A"); err != nil {
				log.Printf("Warning: Failed to update persisted state for %s: %v", record.Hostname, err)
			}

			m.knownHosts[record.Hostname] = true
			syncedCount++

			m.notifier.SendSuccess(fmt.Sprintf("Reconciled DNS: %s -> %s", record.Hostname, expectedIP))
			log.Printf("Reconciliation: Successfully synced %s", record.Hostname)
		}
	}

	log.Printf("Reconciliation complete: %d synced, %d already in sync, %d errors", syncedCount, skippedCount, errorCount)
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
