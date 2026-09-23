package tui

import (
	"context"
	"strings"
	"testing"

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
