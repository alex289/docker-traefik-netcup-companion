package state

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DNSRecord represents a persisted DNS record
type DNSRecord struct {
	Hostname    string    `json:"hostname"`
	Domain      string    `json:"domain"`
	Subdomain   string    `json:"subdomain"`
	IP          string    `json:"ip"`
	RecordType  string    `json:"record_type"`
	LastUpdated time.Time `json:"last_updated"`
}

// State represents the persisted state of DNS records
type State struct {
	Version   int                  `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
	Records   map[string]DNSRecord `json:"records"` // key is the full hostname
}

// Manager handles persistence of DNS state to disk
type Manager struct {
	mu       sync.RWMutex
	filePath string
	state    *State
}

func NewManager(filePath string) (*Manager, error) {
	m := &Manager{
		filePath: filePath,
		state: &State{
			Version: 1,
			Records: make(map[string]DNSRecord),
		},
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Try to load existing state
	if err := m.load(); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning: Failed to load existing state, starting fresh: %v", err)
		}
	}

	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	// Initialize map if nil (for old state files)
	if state.Records == nil {
		state.Records = make(map[string]DNSRecord)
	}

	m.state = &state
	log.Printf("Loaded %d DNS records from state file", len(m.state.Records))
	return nil
}

func (m *Manager) save() error {
	m.state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}

	// Write to temp file first, then rename for atomic write
	tempFile := m.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tempFile, m.filePath); err != nil {
		os.Remove(tempFile) // Clean up temp file on error
		return fmt.Errorf("failed to rename temp state file: %w", err)
	}

	return nil
}

func (m *Manager) UpdateRecord(hostname, domain, subdomain, ip, recordType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := DNSRecord{
		Hostname:    hostname,
		Domain:      domain,
		Subdomain:   subdomain,
		IP:          ip,
		RecordType:  recordType,
		LastUpdated: time.Now(),
	}

	m.state.Records[hostname] = record

	if err := m.save(); err != nil {
		return fmt.Errorf("failed to persist state: %w", err)
	}

	log.Printf("Persisted DNS record state for %s", hostname)
	return nil
}

func (m *Manager) RemoveRecord(hostname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.state.Records, hostname)

	if err := m.save(); err != nil {
		return fmt.Errorf("failed to persist state after removal: %w", err)
	}

	log.Printf("Removed DNS record state for %s", hostname)
	return nil
}

func (m *Manager) GetRecord(hostname string) (DNSRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.state.Records[hostname]
	return record, exists
}

func (m *Manager) GetAllRecords() map[string]DNSRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	records := make(map[string]DNSRecord, len(m.state.Records))
	for k, v := range m.state.Records {
		records[k] = v
	}
	return records
}

// ReconciliationResult represents the result of reconciliation
type ReconciliationResult struct {
	Hostname     string
	Domain       string
	Subdomain    string
	ExpectedIP   string
	ActualIP     string
	Action       string // "create", "update", "in_sync", "not_found"
	NeedsSync    bool
	ErrorMessage string
}

func (m *Manager) GetRecordsForReconciliation() []DNSRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]DNSRecord, 0, len(m.state.Records))
	for _, record := range m.state.Records {
		records = append(records, record)
	}
	return records
}

func (m *Manager) HasRecords() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.state.Records) > 0
}

func (m *Manager) RecordCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.state.Records)
}
