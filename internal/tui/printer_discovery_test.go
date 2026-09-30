package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"niimtui/internal/config"
	"niimtui/internal/transport"
)

func testDiscoveryPrinters() ([]config.PrinterProfile, []config.LabelPreset) {
	return []config.PrinterProfile{
		{Name: "d110", Model: "D110", Transport: "ble", DeviceName: "D110-Test", DefaultPreset: "d110-40x12"},
		{Name: "b1", Model: "B1", Transport: "ble", DeviceName: "B1-Test", DefaultPreset: "b1-50x50"},
	}, []config.LabelPreset{
		{Name: "d110-40x12", WidthMM: 40, HeightMM: 12, Shape: "rect", Layout: "qr-only"},
		{Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-only"},
	}
}

func TestStartupDiscoveryChoosesAdvertisingPrinterWithoutPersisting(t *testing.T) {
	printers, presets := testDiscoveryPrinters()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{Server: config.ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test"}, ActivePrinter: "d110", Printers: printers, Presets: presets}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	session := &trackingPrinterSession{}
	m := NewModelWithPresets(40, 12, "rect", "", PrintConfig{
		ConfigPath: path, Printers: printers, Printer: "d110", Model: "D110", PreferredPrinter: "d110",
		Discover: func(context.Context) ([]transport.ScanResult, error) {
			return []transport.ScanResult{{Name: "B1-Test", Address: "B1-address"}}, nil
		},
		NewSession: func(string) (PrinterSession, error) { return session, nil },
	}, presets, "d110-40x12")
	if m.Print.Session != nil || m.Connection != ConnectionDisconnected {
		t.Fatal("connected before discovery")
	}
	if m.Init() == nil {
		t.Fatal("startup discovery command missing")
	}
	cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq, Results: []transport.ScanResult{{Name: "B1-Test", Address: "B1-address"}}})
	if cmd == nil || m.Print.Printer != "b1" || m.Document.WidthMM != 50 || m.Document.Shape != "round" {
		t.Fatalf("selected printer=%q doc=%#v cmd=%v", m.Print.Printer, m.Document, cmd)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ActivePrinter != "d110" {
		t.Fatalf("auto-selection persisted active printer %q", loaded.ActivePrinter)
	}
}

func TestStartupDiscoveryDoesNotConnectWhenNoPrinterAdvertises(t *testing.T) {
	printers, presets := testDiscoveryPrinters()
	m := NewModelWithPresets(40, 12, "rect", "", PrintConfig{Printers: printers, Printer: "d110", Model: "D110", PreferredPrinter: "d110", NewSession: func(string) (PrinterSession, error) { t.Fatal("created session for offline printer"); return nil, nil }}, presets, "d110-40x12")
	m.Discovering = true
	if cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq}); cmd != nil || m.Print.Session != nil || !strings.Contains(m.Status, "No configured printers") {
		t.Fatalf("offline startup: cmd=%v session=%v status=%q", cmd, m.Print.Session, m.Status)
	}
}

func TestDiscoveryFailureKeepsManualConnectionAvailable(t *testing.T) {
	printers, presets := testDiscoveryPrinters()
	m := NewModelWithPresets(40, 12, "rect", "", PrintConfig{Printers: printers, Printer: "d110", Model: "D110", NewSession: func(string) (PrinterSession, error) { return noopPrinterSession{}, nil }}, presets, "d110-40x12")
	m.Discovering = true
	cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq, Err: errors.New("Bluetooth unavailable")})
	if cmd != nil || m.Print.Session != nil || !strings.Contains(m.Status, "Bluetooth unavailable") {
		t.Fatalf("scan failure state: cmd=%v status=%q", cmd, m.Status)
	}
	if cmd := m.reconnectPrinter(); cmd == nil || m.Connection != ConnectionConnecting {
		t.Fatalf("manual connect after scan failure: cmd=%v state=%s", cmd, m.Connection)
	}
}

func TestDiscoveryTieBreakAndExplicitPrinter(t *testing.T) {
	printers, _ := testDiscoveryPrinters()
	if got, ok := chooseDetectedPrinter(printers, "d110", false); !ok || got.Name != "d110" {
		t.Fatalf("active printer not preferred: %q %t", got.Name, ok)
	}
	if _, ok := chooseDetectedPrinter(printers, "", false); ok {
		t.Fatal("ambiguous printers auto-selected")
	}
	if _, ok := chooseDetectedPrinter(printers[1:], "d110", true); ok {
		t.Fatal("explicit offline printer bypassed discovery")
	}
	if got := detectedPrinters(printers, []transport.ScanResult{{Name: "B1-Test", Address: "B1-address"}, {Name: "Unrelated", Address: "x"}}); len(got) != 1 || got[0].Name != "b1" {
		t.Fatalf("matches = %#v", got)
	}
	printers[0].DeviceName = "Shared"
	printers[1].DeviceName = "Shared"
	printers[1].Identifier = "printer-b1"
	if got := detectedPrinters(printers, []transport.ScanResult{{Name: "Shared", Address: "printer-b1"}}); len(got) != 1 || got[0].Name != "b1" {
		t.Fatalf("ambiguous advertisement should match identifier only: %#v", got)
	}
}

func TestDiscoveryPreservesEditsAndIgnoresStaleResults(t *testing.T) {
	printers, presets := testDiscoveryPrinters()
	m := NewModelWithPresets(40, 12, "rect", "", PrintConfig{Printers: printers, Printer: "d110", Model: "D110", PreferredPrinter: "d110", NewSession: func(string) (PrinterSession, error) { return noopPrinterSession{}, nil }}, presets, "d110-40x12")
	m.Discovering = true
	m.addTextElement()
	cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq, Results: []transport.ScanResult{{Name: "B1-Test"}}})
	if cmd == nil || m.Document.WidthMM != 40 || len(m.Document.Elements) != 1 || m.Preset != -1 {
		t.Fatalf("scan replaced edited document: %#v preset=%d", m.Document, m.Preset)
	}
	m.ScanSeq++
	if cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq - 1, Results: []transport.ScanResult{{Name: "D110-Test"}}}); cmd != nil || m.Print.Printer != "b1" {
		t.Fatal("stale scan changed chosen printer")
	}
}

func TestExplicitOfflinePrinterDoesNotConnectToOtherDetectedDevice(t *testing.T) {
	printers, presets := testDiscoveryPrinters()
	m := NewModelWithPresets(40, 12, "rect", "", PrintConfig{
		Printers: printers, Printer: "d110", Model: "D110", PreferredPrinter: "d110", ExplicitPrinter: true,
		NewSession: func(string) (PrinterSession, error) {
			t.Fatal("connected despite explicit offline choice")
			return nil, nil
		},
	}, presets, "d110-40x12")
	m.Discovering = true
	if cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq, Results: []transport.ScanResult{{Name: "B1-Test"}}}); cmd != nil || m.Print.Printer != "d110" || m.Print.Session != nil {
		t.Fatalf("explicit printer was overridden: %#v, cmd=%v", m.Print, cmd)
	}
}

func TestManualConnectWaitsUntilDiscoveryStops(t *testing.T) {
	printers, presets := testDiscoveryPrinters()
	connects := 0
	m := NewModelWithPresets(40, 12, "rect", "", PrintConfig{Printers: printers, Printer: "d110", Model: "D110", NewSession: func(string) (PrinterSession, error) { connects++; return noopPrinterSession{}, nil }}, presets, "d110-40x12")
	m.Discovering = true
	if cmd := m.reconnectPrinter(); cmd != nil || connects != 0 || !m.PendingConnect {
		t.Fatalf("connected during scan: cmd=%v connects=%d pending=%t", cmd, connects, m.PendingConnect)
	}
	cmd := m.handlePrinterScan(printerScanMsg{Seq: m.ScanSeq, Err: context.Canceled})
	if cmd == nil || connects != 1 || m.Connection != ConnectionConnecting {
		t.Fatalf("manual connect did not run after scan: cmd=%v connects=%d state=%s", cmd, connects, m.Connection)
	}
	if _, ok := cmd().(printerConnectedMsg); !ok {
		t.Fatal("manual connection command did not connect")
	}
}

func TestConnectionResultUpdatesWhileSavePromptOpen(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}})
	m.beginSaveDesignPrompt()
	updated, _ := m.Update(printerConnectedMsg{Info: ConnectionInfo{Meta: map[string]any{"address": "device"}}, Seq: m.ConnectSeq})
	m = updated.(Model)
	if m.Connection != ConnectionConnected || m.ConnectMeta["address"] != "device" || !strings.Contains(m.Status, "Save design preset name") {
		t.Fatalf("connection result lost while prompting: state=%s status=%q", m.Connection, m.Status)
	}
	updated, _ = m.Update(printerConnectionFailedMsg{Err: errors.New("stale"), Seq: m.ConnectSeq - 1})
	if updated.(Model).Connection != ConnectionConnected { t.Fatal("stale connection result changed state") }
}
