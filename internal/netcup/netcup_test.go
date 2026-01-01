package netcup

import (
	"testing"
)

func TestNewNetcupDnsClient(t *testing.T) {
	customerNumber := 12345
	apiKey := "test-api-key"
	apiPassword := "test-api-password"

	client := NewNetcupDnsClient(customerNumber, apiKey, apiPassword)

	if client == nil {
		t.Fatal("NewNetcupDnsClient() returned nil")
	}

	if client.customerNumber != customerNumber {
		t.Errorf("customerNumber = %v, want %v", client.customerNumber, customerNumber)
	}

	if client.apiKey != apiKey {
		t.Errorf("apiKey = %v, want %v", client.apiKey, apiKey)
	}

	if client.apiPassword != apiPassword {
		t.Errorf("apiPassword = %v, want %v", client.apiPassword, apiPassword)
	}

	if client.apiEndpoint != netcupApiEndpointJSON {
		t.Errorf("apiEndpoint = %v, want %v", client.apiEndpoint, netcupApiEndpointJSON)
	}
}

func TestNewNetcupDnsClientWithOptions(t *testing.T) {
	customerNumber := 12345
	apiKey := "test-api-key"
	apiPassword := "test-api-password"
	customEndpoint := "https://custom.endpoint.com/api"
	customRequestId := "custom-request-id"

	opts := &NetcupDnsClientOptions{
		ApiEndpoint:     customEndpoint,
		ClientRequestId: customRequestId,
	}

	client := NewNetcupDnsClientWithOptions(customerNumber, apiKey, apiPassword, opts)

	if client == nil {
		t.Fatal("NewNetcupDnsClientWithOptions() returned nil")
	}

	if client.customerNumber != customerNumber {
		t.Errorf("customerNumber = %v, want %v", client.customerNumber, customerNumber)
	}

	if client.apiKey != apiKey {
		t.Errorf("apiKey = %v, want %v", client.apiKey, apiKey)
	}

	if client.apiPassword != apiPassword {
		t.Errorf("apiPassword = %v, want %v", client.apiPassword, apiPassword)
	}

	if client.apiEndpoint != customEndpoint {
		t.Errorf("apiEndpoint = %v, want %v", client.apiEndpoint, customEndpoint)
	}

	if client.clientRequestId != customRequestId {
		t.Errorf("clientRequestId = %v, want %v", client.clientRequestId, customRequestId)
	}
}

func TestNewNetcupDnsClientWithOptions_PartialOptions(t *testing.T) {
	customerNumber := 12345
	apiKey := "test-api-key"
	apiPassword := "test-api-password"

	// Test with empty endpoint
	opts := &NetcupDnsClientOptions{
		ClientRequestId: "custom-id",
	}

	client := NewNetcupDnsClientWithOptions(customerNumber, apiKey, apiPassword, opts)

	if client.apiEndpoint != netcupApiEndpointJSON {
		t.Errorf("apiEndpoint = %v, want default %v", client.apiEndpoint, netcupApiEndpointJSON)
	}

	if client.clientRequestId != "custom-id" {
		t.Errorf("clientRequestId = %v, want custom-id", client.clientRequestId)
	}

	// Test with empty client request ID
	opts2 := &NetcupDnsClientOptions{
		ApiEndpoint: "https://test.com",
	}

	client2 := NewNetcupDnsClientWithOptions(customerNumber, apiKey, apiPassword, opts2)

	if client2.apiEndpoint != "https://test.com" {
		t.Errorf("apiEndpoint = %v, want https://test.com", client2.apiEndpoint)
	}

	if client2.clientRequestId != "" {
		t.Errorf("clientRequestId = %v, want empty string", client2.clientRequestId)
	}
}

func TestDnsRecord_String(t *testing.T) {
	record := DnsRecord{
		Id:           "123",
		Hostname:     "app",
		Type:         "A",
		Priority:     "0",
		Destination:  "192.168.1.1",
		DeleteRecord: false,
		State:        "yes",
	}

	str := record.String()
	if str == "" {
		t.Error("DnsRecord.String() returned empty string")
	}

	// Check that the string contains key information
	if len(str) < 10 {
		t.Errorf("DnsRecord.String() = %v, seems too short", str)
	}
}

func TestDnsZoneData_String(t *testing.T) {
	zone := DnsZoneData{
		DomainName:   "example.com",
		Ttl:          "300",
		Serial:       "2024010101",
		Refresh:      "28800",
		Retry:        "7200",
		Expire:       "604800",
		DnsSecStatus: true,
	}

	str := zone.String()
	if str == "" {
		t.Error("DnsZoneData.String() returned empty string")
	}

	// Check that the string contains key information
	if len(str) < 10 {
		t.Errorf("DnsZoneData.String() = %v, seems too short", str)
	}
}

func TestNetcupBaseResponseMessage_String(t *testing.T) {
	response := NetcupBaseResponseMessage{
		ServerRequestId: "server-123",
		ClientRequestId: "client-456",
		Action:          "login",
		Status:          "success",
		StatusCode:      2000,
		ShortMessage:    "Login successful",
		LongMessage:     "Login was successful",
	}

	str := response.String()
	if str == "" {
		t.Error("NetcupBaseResponseMessage.String() returned empty string")
	}

	// Check that the string contains key information
	if len(str) < 10 {
		t.Errorf("NetcupBaseResponseMessage.String() = %v, seems too short", str)
	}
}

func TestResponseStatus_Constants(t *testing.T) {
	tests := []struct {
		name   string
		status ResponseStatus
		want   string
	}{
		{"success status", StatusSuccess, "success"},
		{"error status", StatusError, "error"},
		{"started status", StatusStarted, "started"},
		{"pending status", StatusPending, "pending"},
		{"warning status", StatusWarning, "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("Status constant = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

func TestRequestAction_Constants(t *testing.T) {
	tests := []struct {
		name   string
		action RequestAction
		want   string
	}{
		{"login action", actionLogin, "login"},
		{"logout action", actionLogout, "logout"},
		{"infoDnsZone action", actionInfoDnsZone, "infoDnsZone"},
		{"infoDnsRecords action", actionInfoDnsRecords, "infoDnsRecords"},
		{"updateDnsZone action", actionUpdateDnsZone, "updateDnsZone"},
		{"updateDnsRecords action", actionUpdateDnsRecords, "updateDnsRecords"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.action) != tt.want {
				t.Errorf("Action constant = %v, want %v", tt.action, tt.want)
			}
		})
	}
}

func TestNetcupSession_String(t *testing.T) {
	session := &NetcupSession{
		apiSessionId:   "session-123",
		apiKey:         "key-456",
		customerNumber: 12345,
		endpoint:       "https://test.com",
		LastResponse: &NetcupBaseResponseMessage{
			ServerRequestId: "srv-789",
			ClientRequestId: "cli-101",
			Action:          "test",
			Status:          "success",
			StatusCode:      2000,
			ShortMessage:    "Test",
			LongMessage:     "Test message",
		},
	}

	str := session.String()
	if str == "" {
		t.Error("NetcupSession.String() returned empty string")
	}

	if len(str) < 10 {
		t.Errorf("NetcupSession.String() = %v, seems too short", str)
	}
}

func TestDnsRecordSet(t *testing.T) {
	records := []DnsRecord{
		{
			Id:          "1",
			Hostname:    "app",
			Type:        "A",
			Priority:    "0",
			Destination: "192.168.1.1",
		},
		{
			Id:          "2",
			Hostname:    "api",
			Type:        "A",
			Priority:    "0",
			Destination: "192.168.1.2",
		},
	}

	recordSet := DnsRecordSet{
		Content: records,
	}

	if len(recordSet.Content) != 2 {
		t.Errorf("DnsRecordSet.Content length = %d, want 2", len(recordSet.Content))
	}

	if recordSet.Content[0].Hostname != "app" {
		t.Errorf("First record hostname = %v, want app", recordSet.Content[0].Hostname)
	}

	if recordSet.Content[1].Hostname != "api" {
		t.Errorf("Second record hostname = %v, want api", recordSet.Content[1].Hostname)
	}
}

func TestLoginParams(t *testing.T) {
	params := LoginParams{
		CustomerNumber:  12345,
		ApiKey:          "test-key",
		ApiPassword:     "test-password",
		ClientRequestId: "client-123",
	}

	if params.CustomerNumber != 12345 {
		t.Errorf("CustomerNumber = %v, want 12345", params.CustomerNumber)
	}

	if params.ApiKey != "test-key" {
		t.Errorf("ApiKey = %v, want test-key", params.ApiKey)
	}

	if params.ApiPassword != "test-password" {
		t.Errorf("ApiPassword = %v, want test-password", params.ApiPassword)
	}

	if params.ClientRequestId != "client-123" {
		t.Errorf("ClientRequestId = %v, want client-123", params.ClientRequestId)
	}
}

func TestNetcupBaseParams(t *testing.T) {
	params := NetcupBaseParams{
		CustomerNumber:  12345,
		ApiSessionId:    "session-123",
		ApiKey:          "test-key",
		ClientRequestId: "client-456",
	}

	if params.CustomerNumber != 12345 {
		t.Errorf("CustomerNumber = %v, want 12345", params.CustomerNumber)
	}

	if params.ApiSessionId != "session-123" {
		t.Errorf("ApiSessionId = %v, want session-123", params.ApiSessionId)
	}

	if params.ApiKey != "test-key" {
		t.Errorf("ApiKey = %v, want test-key", params.ApiKey)
	}

	if params.ClientRequestId != "client-456" {
		t.Errorf("ClientRequestId = %v, want client-456", params.ClientRequestId)
	}
}
