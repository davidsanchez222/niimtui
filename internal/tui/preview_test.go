package tui

import (
	"strings"
	"testing"

	"niimtui/internal/config"
)

func TestPrintCurrentDocumentKeepsSessionOpen(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}, Copies: 1})
	m.Connection = ConnectionConnected
	cmd := m.printCurrentDocument()
	if cmd == nil {
		t.Fatal("printCurrentDocument() did not start a print")
	}
	msg, ok := cmd().(printResultMsg)
	if !ok || !msg.OK || msg.Err != nil {
		t.Fatalf("printCurrentDocument() = %#v, want successful print", msg)
	}
	cmd = m.handlePrintResult(msg)
	if cmd != nil || m.Connection != ConnectionConnected {
		t.Fatalf("print result scheduled reconnect: command %v, connection %s", cmd, m.Connection)
	}
}

func TestPrintRejectsMismatchedInstalledRoll(t *testing.T) {
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{Session: noopPrinterSession{}, Model: "B1", Copies: 1}, []config.LabelPreset{{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"}}, "b1-50x30")
	m.Connection = ConnectionConnected
	if cmd := m.printCurrentDocument(); cmd != nil || !strings.Contains(m.Status, "does not match") {
		t.Fatalf("print command = %v, status = %q", cmd, m.Status)
	}
}
