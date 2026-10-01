package tui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func selectionCoordinate(t *testing.T, view, value string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(view), "\n") {
		if index := strings.Index(line, value); index >= 0 {
			return ansi.StringWidth(line[:index]), y
		}
	}
	t.Fatalf("%q not visible in view", value)
	return 0, 0
}

func mouseSelectionMsg(action tea.MouseAction, x, y int) tea.MouseMsg {
	return tea.MouseMsg{Action: action, Button: tea.MouseButtonLeft, X: x, Y: y}
}

func TestMouseDragHighlightsCommandAndCopiesFullCommand(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	command := "niimtui print --design 'box'"
	m.Prompt = PromptState{Mode: PromptCommand, Value: command}
	x, y := selectionCoordinate(t, m.View(), "niimtui print")
	updated, _ := m.Update(mouseSelectionMsg(tea.MouseActionPress, x, y))
	m = updated.(Model)
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionMotion, x+len("niimtui print"), y))
	m = updated.(Model)
	if !m.Selection.Active || !strings.Contains(m.View(), "\x1b[7m") {
		t.Fatal("drag did not highlight command")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "clipboard"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	previous := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = previous }()
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionRelease, x+len("niimtui print"), y))
	m = updated.(Model)
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if m.Selection.Active || m.Prompt.Mode != PromptCommand || !strings.Contains(string(data), base64.StdEncoding.EncodeToString([]byte(command))) {
		t.Fatalf("command was not copied on release: active=%t prompt=%q", m.Selection.Active, m.Prompt.Mode)
	}
	if !strings.Contains(m.View(), "\x1b[7m") {
		t.Fatal("highlight disappeared immediately after mouse release")
	}
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionPress, 2, 2))
	m = updated.(Model)
	if strings.Contains(m.View(), "\x1b[7m") {
		t.Fatal("new click did not clear previous highlight")
	}
}

func TestSelectionUsesCellWidthsAndPreservesTyping(t *testing.T) {
	selected := selectedText(textSelection{Start: selectionPoint{1, 0}, End: selectionPoint{3, 0}, Lines: []string{"A界B"}, Active: true, Moved: true})
	if selected != "界" {
		t.Fatalf("wide character selection = %q", selected)
	}
	multiline := selectedText(textSelection{Start: selectionPoint{1, 0}, End: selectionPoint{3, 1}, Lines: []string{"A界B", "xyz"}, Active: true, Moved: true})
	if multiline != "界B\nxyz" {
		t.Fatalf("multiline selection = %q", multiline)
	}
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.addTextElement()
	m.TextBuffer = "hello"
	m.applyTextBuffer(m.TextBuffer)
	m.EditingText = true
	x, y := selectionCoordinate(t, m.View(), "hello"+string(promptCursorRune))
	updated, _ := m.Update(mouseSelectionMsg(tea.MouseActionPress, x, y))
	m = updated.(Model)
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionMotion, x+5, y))
	m = updated.(Model)
	if m.TextBuffer != "hello" || !m.Selection.Moved {
		t.Fatal("highlighting edit popup changed its text")
	}
	updated, _ = m.Update(testKey("a"))
	m = updated.(Model)
	if m.TextBuffer != "helloa" || m.Selection.Active {
		t.Fatalf("ordinary typing failed after selection: buffer=%q", m.TextBuffer)
	}
}

func TestMenuTextCanBeHighlightedWithoutActivatingMenu(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.toggleMenu()
	x, y := selectionCoordinate(t, m.View(), "Auto Insert")
	updated, _ := m.Update(mouseSelectionMsg(tea.MouseActionPress, x, y))
	m = updated.(Model)
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionMotion, x+len("Auto Insert"), y))
	m = updated.(Model)
	if !m.MenuOpen || !strings.Contains(m.View(), "\x1b[7m") {
		t.Fatal("menu drag did not highlight text")
	}
}

func TestHeaderQuitButtonClosesSessionAndReturnsQuit(t *testing.T) {
	session := &trackingPrinterSession{}
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: session})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	x, y := selectionCoordinate(t, m.View(), topBarQuitText())

	updated, _ := m.Update(mouseSelectionMsg(tea.MouseActionPress, x, y))
	m = updated.(Model)
	updated, cmd := m.Update(mouseSelectionMsg(tea.MouseActionRelease, x, y))
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("header quit click returned nil command")
	}
	if session.closes != 1 {
		t.Fatalf("session closes = %d, want 1", session.closes)
	}
}

func TestTopBarMouseMotionUpdatesHoverTarget(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	x, y := selectionCoordinate(t, m.View(), topBarQuitText())

	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, X: x, Y: y})
	m = updated.(Model)

	if m.TopBarHover != topBarQuit {
		t.Fatalf("top bar hover = %d, want quit", m.TopBarHover)
	}
	if !strings.Contains(m.View(), topBarHoverStyle.Render(topBarQuitText())) {
		t.Fatal("hovered quit control is not highlighted")
	}
}

func TestCanvasDraggingStillMovesElements(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.addTextElement()
	element, _ := m.selectedElement()
	x, y := elementClickPoint(m, element)
	updated, _ := m.Update(mouseSelectionMsg(tea.MouseActionPress, x, y))
	m = updated.(Model)
	if m.Selection.Active || m.Drag.Mode == DragNone {
		t.Fatalf("canvas press did not start an element drag: selection=%t drag=%d", m.Selection.Active, m.Drag.Mode)
	}
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionMotion, x+3, y+2))
	m = updated.(Model)
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionRelease, x+3, y+2))
	m = updated.(Model)
	moved, _ := m.selectedElement()
	if moved.XMM == element.XMM && moved.YMM == element.YMM && moved.WidthMM == element.WidthMM && moved.HeightMM == element.HeightMM {
		t.Fatal("canvas mouse drag did not modify the element")
	}
}
