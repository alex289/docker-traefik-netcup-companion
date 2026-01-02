package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.RecordCount() != 0 {
		t.Errorf("Expected 0 records, got %d", manager.RecordCount())
	}
}

func TestUpdateAndGetRecord(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add a record
	err = manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	// Retrieve the record
	record, exists := manager.GetRecord("test.example.com")
	if !exists {
		t.Fatal("Record should exist")
	}

	if record.Hostname != "test.example.com" {
		t.Errorf("Expected hostname 'test.example.com', got '%s'", record.Hostname)
	}
	if record.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", record.Domain)
	}
	if record.Subdomain != "test" {
		t.Errorf("Expected subdomain 'test', got '%s'", record.Subdomain)
	}
	if record.IP != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", record.IP)
	}
	if record.RecordType != "A" {
		t.Errorf("Expected record type 'A', got '%s'", record.RecordType)
	}
}

func TestRemoveRecord(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add and then remove a record
	err = manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	err = manager.RemoveRecord("test.example.com")
	if err != nil {
		t.Fatalf("Failed to remove record: %v", err)
	}

	_, exists := manager.GetRecord("test.example.com")
	if exists {
		t.Error("Record should not exist after removal")
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	// Create first manager and add records
	manager1, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager1.UpdateRecord("test1.example.com", "example.com", "test1", "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	err = manager1.UpdateRecord("test2.example.com", "example.com", "test2", "192.168.1.2", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	// Create second manager and verify records are loaded
	manager2, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}

	if manager2.RecordCount() != 2 {
		t.Errorf("Expected 2 records, got %d", manager2.RecordCount())
	}

	record1, exists := manager2.GetRecord("test1.example.com")
	if !exists {
		t.Fatal("Record test1.example.com should exist")
	}
	if record1.IP != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", record1.IP)
	}

	record2, exists := manager2.GetRecord("test2.example.com")
	if !exists {
		t.Fatal("Record test2.example.com should exist")
	}
	if record2.IP != "192.168.1.2" {
		t.Errorf("Expected IP '192.168.1.2', got '%s'", record2.IP)
	}
}

func TestGetAllRecords(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add multiple records
	manager.UpdateRecord("test1.example.com", "example.com", "test1", "192.168.1.1", "A")
	manager.UpdateRecord("test2.example.com", "example.com", "test2", "192.168.1.2", "A")
	manager.UpdateRecord("app.other.com", "other.com", "app", "10.0.0.1", "A")

	records := manager.GetAllRecords()

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
}

func TestGetRecordsForReconciliation(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add records
	manager.UpdateRecord("test1.example.com", "example.com", "test1", "192.168.1.1", "A")
	manager.UpdateRecord("test2.example.com", "example.com", "test2", "192.168.1.2", "A")

	records := manager.GetRecordsForReconciliation()

	if len(records) != 2 {
		t.Errorf("Expected 2 records for reconciliation, got %d", len(records))
	}
}

func TestHasRecords(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager.HasRecords() {
		t.Error("Should not have records initially")
	}

	manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.1", "A")

	if !manager.HasRecords() {
		t.Error("Should have records after adding one")
	}
}

func TestLastUpdatedTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	beforeUpdate := time.Now()
	time.Sleep(10 * time.Millisecond)

	manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.1", "A")

	record, _ := manager.GetRecord("test.example.com")

	if record.LastUpdated.Before(beforeUpdate) {
		t.Error("LastUpdated should be after the time before update")
	}
}

func TestAtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add a record
	err = manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	// Verify temp file doesn't exist (should have been renamed)
	tempFile := stateFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after successful write")
	}

	// Verify main file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file should exist after write")
	}
}

func TestUpdateExistingRecord(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "test_state.json")

	manager, err := NewManager(stateFile)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Add initial record
	err = manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.1", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	// Update with new IP
	err = manager.UpdateRecord("test.example.com", "example.com", "test", "192.168.1.100", "A")
	if err != nil {
		t.Fatalf("Failed to update record: %v", err)
	}

	record, exists := manager.GetRecord("test.example.com")
	if !exists {
		t.Fatal("Record should exist")
	}

	if record.IP != "192.168.1.100" {
		t.Errorf("Expected updated IP '192.168.1.100', got '%s'", record.IP)
	}

	// Verify only one record exists
	if manager.RecordCount() != 1 {
		t.Errorf("Expected 1 record, got %d", manager.RecordCount())
	}
}
