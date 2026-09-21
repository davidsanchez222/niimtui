package tui

import (
	"runtime"
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

func TestDetectLivePreviewProtocolITerm(t *testing.T) {
	withEnv(t, map[string]string{
		"KITTY_WINDOW_ID": "",
		"TERM_PROGRAM":    "iTerm.app",
		"TERM":            "xterm-256color",
	})

	if got := detectLivePreviewProtocol(); got != LivePreviewITerm2 {
		t.Fatalf("detectLivePreviewProtocol() = %q, want %q", got, LivePreviewITerm2)
	}
}

func TestDetectLivePreviewProtocolMacOpenFallback(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS open fallback is darwin-only")
	}
	withEnv(t, map[string]string{
		"KITTY_WINDOW_ID": "",
		"TERM_PROGRAM":    "Apple_Terminal",
		"TERM":            "xterm-256color",
	})

	if got := detectLivePreviewProtocol(); got != LivePreviewOpen {
		t.Fatalf("detectLivePreviewProtocol() = %q, want %q", got, LivePreviewOpen)
	}
}

func TestTerminalImageEscapeKittyIncludesCellSize(t *testing.T) {
	escape := terminalImageEscape(LivePreviewKitty, []byte{1, 2, 3}, 12, 5)
	if !strings.Contains(escape, "c=12,r=5") {
		t.Fatalf("kitty escape = %q, want cell size", escape)
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
