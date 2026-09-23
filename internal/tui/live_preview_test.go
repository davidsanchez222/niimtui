package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDetectLivePreviewProtocolPrefersKitty(t *testing.T) {
	withEnv(t, map[string]string{
		"KITTY_WINDOW_ID": "1",
		"TERM_PROGRAM":    "iTerm.app",
		"TERM":            "xterm-256color",
	})

	if got := detectLivePreviewProtocol(); got != LivePreviewKitty {
		t.Fatalf("detectLivePreviewProtocol() = %q, want %q", got, LivePreviewKitty)
	}
}

func TestDetectLivePreviewProtocolITermUsesFallback(t *testing.T) {
	withEnv(t, map[string]string{
		"KITTY_WINDOW_ID": "",
		"TERM_PROGRAM":    "iTerm.app",
		"TERM":            "xterm-256color",
	})

	if got := detectLivePreviewProtocol(); got != LivePreviewDisabled {
		t.Fatalf("detectLivePreviewProtocol() = %q, want disabled", got)
	}
}

func TestDetectLivePreviewProtocolDoesNotAutoOpenFallback(t *testing.T) {
	withEnv(t, map[string]string{
		"KITTY_WINDOW_ID": "",
		"TERM_PROGRAM":    "Apple_Terminal",
		"TERM":            "xterm-256color",
	})

	if got := detectLivePreviewProtocol(); got != LivePreviewDisabled {
		t.Fatalf("detectLivePreviewProtocol() = %q, want disabled", got)
	}
}

func TestTerminalImageEscapeKittyIncludesCellSize(t *testing.T) {
	escape := terminalImageEscape(LivePreviewKitty, []byte{1, 2, 3}, 12, 5)
	if !strings.Contains(escape, "c=12,r=5") {
		t.Fatalf("kitty escape = %q, want cell size", escape)
	}
}

func TestTerminalLivePreviewClearDeletesKittyImage(t *testing.T) {
	var b bytes.Buffer
	if _, err := writeTerminalLivePreviewClear(&b, 40); err != nil {
		t.Fatalf("writeTerminalLivePreviewClear() error = %v", err)
	}
	output := b.String()
	if !strings.Contains(output, "a=d,d=I,i=4242") {
		t.Fatalf("clear output = %q, want kitty image delete", output)
	}
	if strings.Contains(output, "a=T") {
		t.Fatalf("clear output = %q, should not draw an image", output)
	}
}

func TestLivePreviewPanelCellSizeUsesDocumentAspect(t *testing.T) {
	square := NewModel(50, 50, "round", "", PrintConfig{})
	wide := NewModel(50, 30, "rect", "", PrintConfig{})
	tall := NewModel(30, 50, "rect", "", PrintConfig{})

	_, squareRows := square.livePreviewPanelCellSize(28)
	_, wideRows := wide.livePreviewPanelCellSize(28)
	_, tallRows := tall.livePreviewPanelCellSize(28)

	if squareRows <= wideRows {
		t.Fatalf("square preview rows = %d, want more than wide rows %d", squareRows, wideRows)
	}
	if tallRows <= squareRows {
		t.Fatalf("tall preview rows = %d, want more than square rows %d", tallRows, squareRows)
	}
}

func TestLivePreviewResizeClearsWhenTerminalTooSmall(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Preview.Protocol = LivePreviewKitty
	m.Preview.PNG = []byte{1, 2, 3}
	m.Width = minTerminalWidth - 1
	m.Height = minTerminalHeight
	m.reflow()

	cmd := m.livePreviewResizeCmd()
	if cmd == nil {
		t.Fatal("expected clear command")
	}
	if m.Preview.RedrawSeq != 0 {
		t.Fatalf("redraw seq = %d, want 0 for immediate clear", m.Preview.RedrawSeq)
	}
}

func TestLivePreviewSchedulesWhenDocumentChanges(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Preview.Protocol = LivePreviewKitty
	m.Preview.LastKey = m.livePreviewKey()
	beforeSeq := m.Preview.RequestedSeq

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model := updated.(Model)
	if model.Preview.RequestedSeq != beforeSeq+1 {
		t.Fatalf("requested seq = %d, want %d", model.Preview.RequestedSeq, beforeSeq+1)
	}
	if cmd == nil {
		t.Fatal("expected live preview debounce command")
	}
}

func withEnv(t *testing.T, values map[string]string) {
	t.Helper()
	previous := getenv
	getenv = func(key string) string {
		return values[key]
	}
	t.Cleanup(func() {
		getenv = previous
	})
}
