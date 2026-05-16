package gui

import (
	"strings"
	"testing"

	"mirage/internal/modes"
)

func TestTrayConnectionTitles(t *testing.T) {
	connect, disconnect := trayConnectionTitles(true)
	if connect != "Connected" || disconnect != "Disconnect" {
		t.Fatalf("titles = %q/%q, want Connected/Disconnect", connect, disconnect)
	}
	connect, disconnect = trayConnectionTitles(false)
	if connect != "Connect" || disconnect != "Disconnected" {
		t.Fatalf("titles = %q/%q, want Connect/Disconnected", connect, disconnect)
	}
}

func TestConnectSuccessMessageIncludesModeAndProxy(t *testing.T) {
	msg := connectSuccessMessage(modes.Normal, true)
	if !strings.Contains(msg, "Connected") {
		t.Fatalf("message = %q, want Connected", msg)
	}
	if !strings.Contains(msg, "normal") {
		t.Fatalf("message = %q, want mode", msg)
	}
	if !strings.Contains(msg, "system proxy enabled") {
		t.Fatalf("message = %q, want proxy status", msg)
	}
}

func TestTrayModeForMenu(t *testing.T) {
	tests := []struct {
		name string
		want modes.Mode
	}{
		{"normal", modes.Normal},
		{"cheap", modes.Cheap},
		{"full", modes.Full},
		{"unknown", modes.Normal},
	}
	for _, tt := range tests {
		if got := trayModeForMenu(tt.name); got != tt.want {
			t.Fatalf("trayModeForMenu(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
