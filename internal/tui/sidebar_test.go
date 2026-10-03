package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"niimtui/internal/config"
)

func sidebarTestModel(t *testing.T, session PrinterSession) Model {
	t.Helper()
	printers := []config.PrinterProfile{
		{Name: "b1-default", Model: "B1", DefaultPreset: "b1-50x30"},
		{Name: "d110-default", Model: "D110", DefaultPreset: "d110-12x40"},
	}
	stocks := []config.LabelPreset{
		{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"},
		{Name: "d110-12x40", WidthMM: 40, HeightMM: 12, Shape: "rect"},
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{
		ConfigPath: t.TempDir() + "/missing.json",
		Printers:   printers, Printer: "b1-default", Model: "B1",
		Session:    session,
		NewSession: func(string) (PrinterSession, error) { return &trackingPrinterSession{}, nil },
	}, stocks, "b1-50x30")
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.focusSidebar()
	return m
}

// renderedPosition finds text in the rendered screen and returns its column and row.
func renderedPosition(t *testing.T, m Model, text string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(m.View()), "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return len([]rune(line[:i])), y
		}
	}
	t.Fatalf("%q not found in view", text)
	return 0, 0
}

func clickAt(m Model, x, y int) (Model, tea.Cmd) {
	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	m = updated.(Model)
	updated, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: x, Y: y})
	return updated.(Model), cmd
}

func TestSidebarChipStates(t *testing.T) {
	session := &trackingPrinterSession{}
	m := sidebarTestModel(t, session)
	active := m.Print.Printers[0]
	other := m.Print.Printers[1]

	for _, tc := range []struct {
		state  ConnectionStatus
		text   string
		action chipAction
	}{
		{ConnectionConnected, "● connected", chipDisconnect},
		{ConnectionConnecting, "◐ connecting", chipNone},
		{ConnectionDisconnected, "○ reconnect", chipReconnect},
	} {
		m.Connection = tc.state
		text, _, action := sidebarChip(m, active)
		if text != tc.text || action != tc.action {
			t.Fatalf("%s chip = %q/%v, want %q/%v", tc.state, text, action, tc.text, tc.action)
		}
	}
	if text, _, action := sidebarChip(m, other); text != "○ connect" || action != chipConnect {
		t.Fatalf("unknown scan chip = %q/%v", text, action)
	}
	m.DetectedNames = map[string]bool{"d110-default": false}
	if text, _, _ := sidebarChip(m, other); text != "· not seen" {
		t.Fatalf("not seen chip = %q", text)
	}
	m.DetectedNames["d110-default"] = true
	if text, _, _ := sidebarChip(m, other); text != "○ connect" {
		t.Fatalf("seen chip = %q", text)
	}
}

func TestSidebarSpacerIsSkippedByNavigation(t *testing.T) {
	m := sidebarTestModel(t, &trackingPrinterSession{})
	m.SidebarIndex = 1 // b1 roll, the last row before the spacer
	m.handleSidebarKey(testKey("j"))
	row, _ := m.currentSidebarRow()
	if !row.Header || row.Printer.Name != "d110-default" {
		t.Fatalf("j landed on %+v, want the d110 header", row)
	}
	m.handleSidebarKey(testKey("k"))
	if row, _ = m.currentSidebarRow(); row.Spacer || row.Header {
		t.Fatalf("k landed on %+v, want the b1 roll", row)
	}
}

func TestSidebarHeaderRendersChipRightAligned(t *testing.T) {
	m := sidebarTestModel(t, &trackingPrinterSession{})
	m.Connection = ConnectionConnected
	lines := sidebarLines(m, panelContentWidth(layoutLeftPanelWidth))
	header := ansi.Strip(lines[2])
	if !strings.Contains(header, "b1-default") || !strings.HasSuffix(strings.TrimRight(header, " "), "● connected") {
		t.Fatalf("header = %q", header)
	}
}

func TestClickConnectedChipDisconnects(t *testing.T) {
	session := &trackingPrinterSession{}
	m := sidebarTestModel(t, session)
	m.Connection = ConnectionConnected
	x, y := renderedPosition(t, m, "● connected")
	m, _ = clickAt(m, x+2, y)
	if session.closes != 1 || m.Connection != ConnectionDisconnected {
		t.Fatalf("closes=%d state=%s, want disconnect", session.closes, m.Connection)
	}
}

func TestClickConnectChipOnOtherPrinterSwitchesAndConnects(t *testing.T) {
	m := sidebarTestModel(t, &trackingPrinterSession{})
	m.Connection = ConnectionConnected
	x, y := renderedPosition(t, m, "○ connect")
	var cmd tea.Cmd
	m, cmd = clickAt(m, x+1, y)
	if m.Print.Printer != "d110-default" || cmd == nil {
		t.Fatalf("printer=%q cmd=%v, want switch to d110 and a connect command", m.Print.Printer, cmd)
	}
}

func TestClickHeaderNameFoldsAndRollSelects(t *testing.T) {
	m := sidebarTestModel(t, &trackingPrinterSession{})
	m.Connection = ConnectionConnected
	x, y := renderedPosition(t, m, "b1-default")
	m, _ = clickAt(m, x, y)
	if !m.SidebarCollapsed["b1-default"] {
		t.Fatal("clicking the header name did not fold the section")
	}
	m, _ = clickAt(m, x, y)
	if m.SidebarCollapsed["b1-default"] {
		t.Fatal("second click did not expand the section")
	}
	rx, ry := renderedPosition(t, m, "d110-12x40")
	m, _ = clickAt(m, rx, ry)
	if m.Print.Printer != "d110-default" {
		t.Fatalf("clicking a roll on another printer left printer=%q", m.Print.Printer)
	}
}

func TestClickUnfocusedPrinterPanelFocusesSidebar(t *testing.T) {
	m := sidebarTestModel(t, &trackingPrinterSession{})
	m.SidebarFocused = false
	m, _ = clickAt(m, 5, sidebarFirstRow)
	if !m.SidebarFocused {
		t.Fatal("clicking the printer panel did not focus the sidebar")
	}
}

func TestSidebarFirstRowMatchesRenderedHeader(t *testing.T) {
	m := sidebarTestModel(t, &trackingPrinterSession{})
	if _, y := renderedPosition(t, m, "b1-default"); y != sidebarFirstRow {
		t.Fatalf("first row rendered at y=%d, sidebarFirstRow=%d", y, sidebarFirstRow)
	}
	chipX, _ := renderedPosition(t, m, "○ connect")
	start, _ := sidebarChipSpan(m, m.Print.Printers[1], panelContentWidth(layoutLeftPanelWidth))
	if chipX != start+sidebarContentX {
		t.Fatalf("chip rendered at x=%d, computed %d", chipX, start+sidebarContentX)
	}
}
