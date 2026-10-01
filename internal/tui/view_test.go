package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func TestViewHeightStaysWithinTerminalAfterRotatingCanvasWithSelection(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Model: "B1"})
	m.Ready = true
	m.Width = 200
	m.Height = 32
	m.reflow()
	m.addTextElement()
	if !m.beginEditingSelected() {
		t.Fatal("beginEditingSelected() = false, want true")
	}
	m.TextBuffer = "hello"
	_ = m.applyTextBuffer(m.TextBuffer)
	m.finishTextEdit()
	if !m.rotateCanvas() {
		t.Fatal("rotateCanvas() = false, want true")
	}

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > m.Height {
		t.Fatalf("view height = %d, want <= terminal height %d", len(lines), m.Height)
	}
	if !strings.Contains(view, "┌") {
		t.Fatal("rotated canvas top border missing from view")
	}
}

func TestFontPickerSearchVisibleInShortSidebar(t *testing.T) {
	m := NewModel(50, 50, "round", "", PrintConfig{})
	m.Ready = true
	m.Width = 200
	m.Height = minTerminalHeight
	m.reflow()
	m.addTextElement()
	m.Fonts = []FontOption{
		{Name: "Default", Path: ""},
		{Name: "Go-Regular", Path: "/tmp/Go-Regular.ttf"},
		{Name: "JetBrainsMonoNerdFont-Regular", Path: "/tmp/JetBrainsMonoNerdFont-Regular.ttf"},
	}
	m.FontPickerOpen = true
	m.FontPickerSearch = true
	m.FontPickerQuery = "jet"
	m.Preview.Protocol = LivePreviewKitty
	m.Preview.PNG = []byte{1, 2, 3}

	lines := fitPanelLines(propertyPanelLines(m, layoutPropertiesWidth), m.canvasPanelHeight(), layoutPropertiesWidth)
	panel := strings.Join(lines, "\n")
	if !strings.Contains(panel, "Search") || !strings.Contains(panel, "jet") {
		t.Fatalf("short property panel = %q, want visible font search", panel)
	}
}

func TestEditingTextRendersPopupWithBlockCursor(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Ready = true
	m.Width = minTerminalWidth
	m.Height = minTerminalHeight
	m.reflow()
	m.addTextElement()
	m.TextBuffer = "hello"
	_ = m.applyTextBuffer(m.TextBuffer)
	m.EditingText = true
	m.refreshStatus()

	view := m.View()
	if !strings.Contains(view, "hello"+string(promptCursorRune)) {
		t.Fatalf("view = %q, want text followed by block cursor %q", view, promptCursorRune)
	}
	if strings.Contains(view, "Edit Text") || strings.Contains(view, "type to edit") {
		t.Fatalf("view = %q, should not render extra edit popup copy", view)
	}
	if strings.Contains(view, "hello|") {
		t.Fatalf("view = %q, should not render pipe cursor", view)
	}
	if !strings.Contains(view, "Properties") || !strings.Contains(view, "Printer") {
		t.Fatalf("view = %q, want side panels visible while editing", view)
	}

	canvas := renderCanvas(m)
	if strings.Contains(canvas, "|") {
		t.Fatalf("canvas = %q, should not render in-component pipe cursor", canvas)
	}
}

func TestOverlayStyledLineKeepsBackgroundAroundTextbox(t *testing.T) {
	line := overlayStyledLine("abcdef", "XY", 2, 6)
	if line != "abXYef" {
		t.Fatalf("overlay line = %q, want abXYef", line)
	}
}

func TestEditingTextLinesKeepCursorVisibleForLongText(t *testing.T) {
	lines := editingTextLines(strings.Repeat("a", 80), 12, 2)
	joined := strings.Join(lines, "\n")
	if !strings.ContainsRune(joined, promptCursorRune) {
		t.Fatalf("editing lines = %q, want visible block cursor %q", joined, promptCursorRune)
	}
}

func TestSwitchPresetUpdatesDocumentAndCanvas(t *testing.T) {
	presets := []config.LabelPreset{
		{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"},
		{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round"},
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{Printers: []config.PrinterProfile{{Name: "b1", Model: "B1"}}, Printer: "b1", Model: "B1"}, presets, "b1-50x30")
	m.Width = 160
	m.Height = 30
	m.reflow()

	m.focusSidebar()
	m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyEnter})
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

	m.focusSidebar()
	m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyDown})
	cmd := m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("focused printer selection returned nil, want connect command")
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

func TestFocusedSidebarSelectsPrinter(t *testing.T) {
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

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if !m.SidebarFocused {
		t.Fatal("Tab did not focus printer sidebar")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.Print.Printer != "d110-default" || m.Print.Model != "D110" {
		t.Fatalf("active printer = %q %q, want d110-default D110", m.Print.Printer, m.Print.Model)
	}
}

func TestPrinterSidebarKeysAreContextual(t *testing.T) {
	presets := []config.LabelPreset{{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"}, {Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "round"}}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{Printers: []config.PrinterProfile{{Name: "b1", Model: "B1"}}, Printer: "b1", Model: "B1"}, presets, "b1-50x30")
	updated, _ := m.Update(testKey("n"))
	m = updated.(Model)
	if m.Preset != 0 {
		t.Fatal("legacy global roll shortcut still active")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(testKey("h"))
	m = updated.(Model)
	if !m.SidebarCollapsed["b1"] || len(m.sidebarRows()) != 1 {
		t.Fatal("h did not collapse the printer's rolls")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.SidebarCollapsed["b1"] {
		t.Fatal("right arrow did not expand printer rolls")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.Preset != 1 || m.Document.HeightMM != 50 || m.Document.Shape != "round" {
		t.Fatalf("sidebar roll selection: index=%d doc=%#v", m.Preset, m.Document)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.SidebarFocused {
		t.Fatal("Esc did not return focus to the designer")
	}
}

func TestPrinterTreeCanSelectRollOnAnotherPrinter(t *testing.T) {
	printers := []config.PrinterProfile{{Name: "b1", Model: "B1", DefaultPreset: "b1-round"}, {Name: "d110", Model: "D110", DefaultPreset: "d110-small"}}
	stocks := []config.LabelPreset{{Name: "b1-round", WidthMM: 50, HeightMM: 50, Shape: "round"}, {Name: "d110-small", WidthMM: 40, HeightMM: 12, Shape: "rect"}}
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{ConfigPath: filepath.Join(t.TempDir(), "missing.json"), Printers: printers, Printer: "b1", Model: "B1", NewSession: func(string) (PrinterSession, error) { return noopPrinterSession{}, nil }}, stocks, "b1-round")
	m.focusSidebar()
	if got := len(m.sidebarRows()); got != 4 {
		t.Fatalf("tree has %d rows, want two printers and two rolls", got)
	}
	m.handleSidebarKey(testKey("j")) // B1 roll
	m.handleSidebarKey(testKey("j")) // D110 printer
	m.handleSidebarKey(testKey("j")) // D110 roll
	cmd := m.handleSidebarKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.Print.Printer != "d110" || m.Preset != 0 || m.Document.HeightMM != 12 {
		t.Fatalf("roll selection did not switch printer and stock: cmd=%v printer=%q preset=%d", cmd, m.Print.Printer, m.Preset)
	}
}

func TestPrinterPanelShowsConfiguredPrintersWithoutProfile(t *testing.T) {
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
		t.Fatalf("printer panel = %q, want printer display names", panel)
	}
}

func TestConnectHelpRendersInPrinterPanel(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}})
	m.Connection = ConnectionDisconnected
	m.focusSidebar()
	leftPanel := strings.Join(sidebarLines(m, layoutLeftPanelWidth), "\n")
	if !strings.Contains(leftPanel, "reconnect") {
		t.Fatalf("printer panel = %q, want reconnect help", leftPanel)
	}

	footer := strings.Join(footerLines(m, minTerminalWidth), "\n")
	if strings.Contains(footer, "reconnect") {
		t.Fatalf("footer = %q, want reconnect help moved out", footer)
	}
}

func TestFocusedBindingLegend(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	legend := strings.Join(footerLines(m, minTerminalWidth), "\n")
	if strings.Contains(legend, "binding") || strings.Contains(legend, "required") {
		t.Fatalf("binding hints shown without selection: %q", legend)
	}
	m.addTextElement()
	m.Print.Session = noopPrinterSession{}
	legend = strings.Join(footerLines(m, minTerminalWidth), "\n")
	for _, line := range footerLines(m, minTerminalWidth) {
		if width := lipgloss.Width(line); width > minTerminalWidth {
			t.Fatalf("focused legend is %d cells, exceeds minimum width %d", width, minTerminalWidth)
		}
	}
	properties := strings.Join(propertyPanelLines(m, layoutPropertiesWidth), "\n")
	if !strings.Contains(legend, "binding") || !strings.Contains(legend, "required") || !strings.Contains(properties, "name binding") || !strings.Contains(properties, "search fonts") {
		t.Fatalf("text hints missing: legend=%q properties=%q", legend, properties)
	}
	m.addQRElement()
	properties = strings.Join(propertyPanelLines(m, layoutPropertiesWidth), "\n")
	if !strings.Contains(properties, "name binding") || strings.Contains(properties, "search fonts") {
		t.Fatalf("QR hints incorrect: %q", properties)
	}
	m.SelectedID = ""
	legend = strings.Join(footerLines(m, minTerminalWidth), "\n")
	if strings.Contains(legend, "binding") || strings.Contains(legend, "required") {
		t.Fatalf("binding hints still shown after clearing selection: %q", legend)
	}
}

func TestNavigationLegendsShowArrowAndVimKeys(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Printers: []config.PrinterProfile{{Name: "b1", Model: "B1"}}, Printer: "b1", Model: "B1"})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.focusSidebar()
	if legend := strings.Join(sidebarLines(m, layoutLeftPanelWidth), "\n") + strings.Join(footerLines(m, minTerminalWidth), "\n"); !strings.Contains(legend, "↑/↓ k/j") || !strings.Contains(legend, "←/→ h/l") {
		t.Fatalf("printer tree hints missing arrow/Vim pairs: %q", legend)
	}
	m.Tab = tabGallery
	for _, line := range footerLines(m, minTerminalWidth) {
		if width := lipgloss.Width(line); width > minTerminalWidth {
			t.Fatalf("gallery legend exceeds terminal width: %d", width)
		}
	}
	if legend := strings.Join(footerLines(m, minTerminalWidth), "\n"); !strings.Contains(legend, "↑/↓ k/j") || !strings.Contains(legend, "←/→ h/l") {
		t.Fatalf("gallery hints missing arrow/Vim pairs: %q", legend)
	}
	m.Prompt = PromptState{Mode: PromptOverwriteDesign, Value: "existing"}
	if modal := strings.Join(modalBodyLines(m, minTerminalWidth, m.canvasPanelHeight()), "\n"); !strings.Contains(modal, "↑/↓ or k/j") {
		t.Fatalf("confirmation hints missing arrow/Vim pair: %q", modal)
	}
}

func TestDisconnectShortcutClosesSessionAndAllowsReconnect(t *testing.T) {
	session := &trackingPrinterSession{}
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: session})
	m.Connection = ConnectionConnected
	m.ConnectMeta = map[string]any{"connected": true}
	m.focusSidebar()
	panel := strings.Join(sidebarLines(m, layoutLeftPanelWidth), "\n")
	if !strings.Contains(panel, "disconnect") || !strings.Contains(panel, "D") {
		t.Fatalf("disconnect shortcut missing in printer panel: %q", panel)
	}
	updated, cmd := m.update(testKey("D"))
	if cmd != nil || session.closes != 1 || updated.Connection != ConnectionDisconnected || updated.ConnectMeta != nil {
		t.Fatalf("disconnect state: cmd=%v closes=%d state=%s meta=%v", cmd, session.closes, updated.Connection, updated.ConnectMeta)
	}
	if panel = strings.Join(sidebarLines(updated, layoutLeftPanelWidth), "\n"); !strings.Contains(panel, "c reconnect") || strings.Contains(panel, "D disconnect") {
		t.Fatalf("reconnect shortcut missing after disconnect: %q", panel)
	}
	updated, cmd = updated.update(testKey("c"))
	if cmd == nil || updated.Connection != ConnectionConnecting {
		t.Fatalf("reconnect command=%v state=%s", cmd, updated.Connection)
	}
	if _, ok := cmd().(printerConnectedMsg); !ok || session.connects != 1 {
		t.Fatalf("session was not reconnected: %d", session.connects)
	}
}

type trackingPrinterSession struct{ closes, connects int }

func (s *trackingPrinterSession) Connect(context.Context) (map[string]any, error) {
	s.connects++
	return map[string]any{"connected": true}, nil
}
func (*trackingPrinterSession) PrintImage(context.Context, render.Result, int) api.PrintResponse {
	return api.PrintResponse{OK: true}
}
func (*trackingPrinterSession) Metadata() map[string]any { return nil }
func (s *trackingPrinterSession) Close() error           { s.closes++; return nil }

func TestPrintResultKeepsConnectedPrinter(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}})
	m.Connection = ConnectionConnected

	cmd := m.handlePrintResult(printResultMsg{OK: true, Printer: "test", Copies: 1, WidthPx: 384, HeightPx: 240})
	if cmd != nil {
		t.Fatal("handlePrintResult() returned a reconnect command")
	}
	if m.Connection != ConnectionConnected {
		t.Fatalf("connection = %s, want connected", m.Connection)
	}
	if !strings.Contains(m.Status, "Printed 1 copy") || strings.Contains(m.Status, "Reconnecting") {
		t.Fatalf("status = %q, want printed and connected status", m.Status)
	}
}

func TestPrintResultShowsDisconnectedAfterFailedRetry(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}})
	m.Connection = ConnectionConnected
	m.handlePrintResult(printResultMsg{Err: errors.New("connection lost"), Disconnected: true})
	if m.Connection != ConnectionDisconnected {
		t.Fatalf("connection = %s, want disconnected", m.Connection)
	}
}

type noopPrinterSession struct{}

func (noopPrinterSession) Connect(context.Context) (map[string]any, error) { return nil, nil }

func (noopPrinterSession) PrintImage(context.Context, render.Result, int) api.PrintResponse {
	return api.PrintResponse{OK: true}
}

func (noopPrinterSession) Close() error { return nil }

func (noopPrinterSession) Metadata() map[string]any { return map[string]any{"connected": true} }

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
