package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/render"
)

func TestDrawPrintableAreaGuideSkipsRoundLabels(t *testing.T) {
	canvas := newCanvas(80, 24, 50, 50)
	m := NewModel(50, 50, "round", "", PrintConfig{Model: "B1"})
	grid := newTestGrid(canvas)

	drawPrintableAreaGuide(grid, canvas, m)

	if guideCount := countNonZeroRunes(grid); guideCount != 0 {
		t.Fatalf("round printable area guide drew %d cells, want none", guideCount)
	}

	m.Document.Shape = "rect"
	drawPrintableAreaGuide(grid, canvas, m)
	if guideCount := countNonZeroRunes(grid); guideCount == 0 {
		t.Fatal("rect printable area guide did not draw any cells")
	}
}

func TestDrawPrintDirectionGuideShowsModelDirection(t *testing.T) {
	canvas := Canvas{Width: 12, Height: 8}
	grid := newTestGrid(canvas)
	drawPrintDirectionGuide(grid, canvas, NewModel(50, 30, "rect", "", PrintConfig{Model: "D110"}))
	if grid[canvas.Height/2][canvas.Width-1] != '▶' {
		t.Fatalf("D110 print direction marker = %q, want ▶", grid[canvas.Height/2][canvas.Width-1])
	}

	grid = newTestGrid(canvas)
	drawPrintDirectionGuide(grid, canvas, NewModel(50, 30, "rect", "", PrintConfig{Model: "B1"}))
	if grid[0][canvas.Width/2] != '▲' {
		t.Fatalf("B1 print direction marker = %q, want ▲", grid[0][canvas.Width/2])
	}
}

func TestReflowCentersCanvasInPanel(t *testing.T) {
	m := NewModel(50, 50, "round", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()

	wantX := m.canvasPanelLeft() + (m.canvasPanelWidth()-m.Canvas.Width)/2
	if m.Canvas.X != wantX {
		t.Fatalf("canvas x = %d, want centered x %d", m.Canvas.X, wantX)
	}
}

func TestReflowCentersWideCanvasVertically(t *testing.T) {
	m := NewModel(80, 30, "rect", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()

	wantY := layoutBodyTop + (m.canvasPanelHeight()-m.Canvas.Height)/2
	if m.Canvas.Y != wantY {
		t.Fatalf("canvas y = %d, want centered y %d", m.Canvas.Y, wantY)
	}
}

func TestSwitchPresetUpdatesDocumentAndCanvas(t *testing.T) {
	presets := []config.LabelPreset{
		{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"},
		{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round"},
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{}, presets, "b1-50x30")
	m.Width = 160
	m.Height = 30
	m.reflow()

	if !m.switchPreset(1) {
		t.Fatal("switchPreset() = false, want true")
	}
	if m.Document.WidthMM != 50 || m.Document.HeightMM != 50 || m.Document.Shape != "round" {
		t.Fatalf("document = %.0fx%.0f %s, want 50x50 round", m.Document.WidthMM, m.Document.HeightMM, m.Document.Shape)
	}
	if m.Preset != 1 {
		t.Fatalf("preset index = %d, want 1", m.Preset)
	}
	wantX := m.canvasPanelLeft() + (m.canvasPanelWidth()-m.Canvas.Width)/2
	if m.Canvas.X != wantX {
		t.Fatalf("canvas x = %d, want centered x %d", m.Canvas.X, wantX)
	}
}

func TestSwitchPrinterFiltersPresetsAndSavesActivePrinter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{
		Server:        config.ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test-token"},
		ActivePrinter: "b1-default",
		Printers: []config.PrinterProfile{
			{Name: "b1-default", Model: "B1", Transport: "ble", DeviceName: "B1-Test", DefaultPreset: "b1-50x30"},
			{Name: "d110-default", Model: "D110", Transport: "ble", DeviceName: "D110-Test", DefaultPreset: "d110-12x40"},
		},
		Presets: []config.LabelPreset{
			{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-title", MarginsMM: 2},
			{Name: "d110-12x40", WidthMM: 40, HeightMM: 12, Shape: "rect", Layout: "qr-only", MarginsMM: 1},
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{
		ConfigPath: path,
		Printers:   cfg.Printers,
		Printer:    "b1-default",
		Model:      "B1",
		NewSession: func(string) (PrinterSession, error) { return noopPrinterSession{}, nil },
	}, cfg.Presets, "b1-50x30")

	cmd := m.switchPrinterIndex(1)
	if cmd == nil {
		t.Fatal("switchPrinterIndex() returned nil command, want connect command")
	}
	if m.Print.Printer != "d110-default" || m.Print.Model != "D110" {
		t.Fatalf("active printer = %q %q, want d110-default D110", m.Print.Printer, m.Print.Model)
	}
	if len(m.Presets) != 1 || m.Presets[0].Name != "d110-12x40" {
		t.Fatalf("visible presets = %#v, want only d110-12x40", m.Presets)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.ActivePrinter != "d110-default" {
		t.Fatalf("active_printer = %q, want d110-default", loaded.ActivePrinter)
	}
}

func TestNumberKeySwitchesPrinter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{
		Server:        config.ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test-token"},
		ActivePrinter: "b1-default",
		Printers: []config.PrinterProfile{
			{Name: "b1-default", Model: "B1", Transport: "ble", DeviceName: "B1-Test", DefaultPreset: "b1-50x30"},
			{Name: "d110-default", Model: "D110", Transport: "ble", DeviceName: "D110-Test", DefaultPreset: "d110-12x40"},
		},
		Presets: []config.LabelPreset{
			{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-title", MarginsMM: 2},
			{Name: "d110-12x40", WidthMM: 40, HeightMM: 12, Shape: "rect", Layout: "qr-only", MarginsMM: 1},
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{ConfigPath: path, Printers: cfg.Printers, Printer: "b1-default", Model: "B1"}, cfg.Presets, "b1-50x30")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = updated.(Model)
	if m.Print.Printer != "d110-default" || m.Print.Model != "D110" {
		t.Fatalf("active printer = %q %q, want d110-default D110", m.Print.Printer, m.Print.Model)
	}
}

func TestPrinterPanelShowsNumberedPrintersWithoutProfile(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{
		Printers: []config.PrinterProfile{
			{Name: "b1-default", Model: "B1", DeviceName: "B1-Test"},
			{Name: "d110-default", Model: "D110", DeviceName: "D110-Test"},
		},
		Printer:    "b1-default",
		Model:      "B1",
		DeviceName: "B1-Test",
	})
	panel := strings.Join(devicePanelLines(m, layoutLeftPanelWidth), "\n")
	if strings.Contains(panel, "Profile") {
		t.Fatalf("printer panel = %q, should not contain Profile", panel)
	}
	if !strings.Contains(panel, "B1-Test (B1)") || !strings.Contains(panel, "D110-Test (D110)") {
		t.Fatalf("printer panel = %q, want numbered display names", panel)
	}
}

func TestConnectHelpRendersInPrinterPanel(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}})
	m.Connection = ConnectionDisconnected
	leftPanel := strings.Join(devicePanelLines(m, layoutLeftPanelWidth), "\n")
	if !strings.Contains(leftPanel, "reconnect") {
		t.Fatalf("printer panel = %q, want reconnect help", leftPanel)
	}

	footer := strings.Join(footerLines(m, minTerminalWidth), "\n")
	if strings.Contains(footer, "reconnect") {
		t.Fatalf("footer = %q, want reconnect help moved out", footer)
	}
}

func TestPrintResultReconnectsAfterClosedPrint(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}})
	m.Connection = ConnectionConnected

	cmd := m.handlePrintResult(printResultMsg{OK: true, Printer: "test", Copies: 1, WidthPx: 384, HeightPx: 240, Closed: true})
	if cmd == nil {
		t.Fatal("handlePrintResult() returned nil command, want reconnect command")
	}
	if m.Connection != ConnectionConnecting {
		t.Fatalf("connection = %s, want connecting", m.Connection)
	}
	if !strings.Contains(m.Status, "Printed 1 copy") || !strings.Contains(m.Status, "Reconnecting") {
		t.Fatalf("status = %q, want printed reconnecting status", m.Status)
	}
	msg := cmd()
	if _, ok := msg.(printerConnectedMsg); !ok {
		t.Fatalf("reconnect command msg = %T, want printerConnectedMsg", msg)
	}
}

type noopPrinterSession struct{}

func (noopPrinterSession) Connect(context.Context) (map[string]any, error) { return nil, nil }

func (noopPrinterSession) PrintImage(context.Context, render.Result, int) api.PrintResponse {
	return api.PrintResponse{OK: true}
}

func (noopPrinterSession) Close() error { return nil }

func newTestGrid(canvas Canvas) [][]rune {
	grid := make([][]rune, canvas.Height)
	for y := range grid {
		grid[y] = make([]rune, canvas.Width)
	}
	return grid
}

func countNonZeroRunes(grid [][]rune) int {
	count := 0
	for _, row := range grid {
		for _, r := range row {
			if r != 0 {
				count++
			}
		}
	}
	return count
}
