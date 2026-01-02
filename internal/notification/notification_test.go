package notification

import (
	"testing"
)

func TestNewNotifier(t *testing.T) {
	tests := []struct {
		name    string
		urls    []string
		enabled bool
	}{
		{
			name:    "no URLs",
			urls:    []string{},
			enabled: false,
		},
		{
			name:    "single URL",
			urls:    []string{"generic://example.com"},
			enabled: true,
		},
		{
			name:    "multiple URLs",
			urls:    []string{"generic://example.com", "generic://example.org"},
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNotifier(tt.urls)
			if n.enabled != tt.enabled {
				t.Errorf("NewNotifier() enabled = %v, want %v", n.enabled, tt.enabled)
			}
			if tt.enabled && n.sender == nil {
				t.Errorf("NewNotifier() sender should not be nil when enabled")
			}
		})
	}
}

func TestNotifier_SendWhenDisabled(t *testing.T) {
	n := NewNotifier([]string{})

	// These should not panic even when disabled
	n.SendSuccess("test")
	n.SendError("test")
	n.SendInfo("test")
}
