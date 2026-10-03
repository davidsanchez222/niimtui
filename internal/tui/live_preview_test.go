package tui

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
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

func TestDetectLivePreviewProtocolWezTerm(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"term program": {"TERM_PROGRAM": "WezTerm", "TERM": "xterm-256color"},
		"pane var":     {"TERM_PROGRAM": "tmux", "TERM": "screen", "WEZTERM_PANE": "3"},
	} {
		t.Run(name, func(t *testing.T) {
			withEnv(t, env)
			if got := detectLivePreviewProtocol(); got != LivePreviewKitty {
				t.Fatalf("detectLivePreviewProtocol() = %q, want %q", got, LivePreviewKitty)
			}
		})
	}
}

func TestDetectLivePreviewProtocolOverride(t *testing.T) {
	withEnv(t, map[string]string{"NIIMTUI_GRAPHICS": "kitty", "TERM": "xterm-256color"})
	if got := detectLivePreviewProtocol(); got != LivePreviewKitty {
		t.Fatalf("override kitty: got %q", got)
	}
	withEnv(t, map[string]string{"NIIMTUI_GRAPHICS": "off", "KITTY_WINDOW_ID": "1"})
	if got := detectLivePreviewProtocol(); got != LivePreviewDisabled {
		t.Fatalf("override off: got %q", got)
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

func TestKittyPreviewCommandsSuppressTerminalResponses(t *testing.T) {
	var output bytes.Buffer
	// Enough data to require multiple graphics commands, not just a first chunk.
	if _, err := writeTerminalLivePreview(&output, LivePreviewKitty, bytes.Repeat([]byte{42}, 9000), 40, 12, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := writeTerminalLivePreviewClear(&output, 40); err != nil {
		t.Fatal(err)
	}
	commands := strings.Split(output.String(), "\x1b_G")[1:]
	if len(commands) < 4 {
		t.Fatalf("got %d graphics commands, want delete, multiple chunks, and clear", len(commands))
	}
	for _, command := range commands {
		controls, _, ok := strings.Cut(command, ";")
		if !ok || !strings.Contains(controls, ",q=2") {
			t.Fatalf("graphics command requests a terminal reply: %q", controls)
		}
	}
}

func TestKittyPreviewDrawsInsideInspectorContent(t *testing.T) {
	var output bytes.Buffer
	if _, err := writeTerminalLivePreview(&output, LivePreviewKitty, []byte{1, 2, 3}, 40, 12, 5); err != nil {
		t.Fatal(err)
	}

	row := layoutBodyTop + 2
	col := layoutLeftPanelWidth + layoutPanelGap + 40 + 2 + layoutPanelGap + 2
	want := "\x1b[" + strconv.Itoa(row) + ";" + strconv.Itoa(col) + "H"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("kitty preview position = %q, want cursor move %q", output.String(), want)
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

func TestLivePreviewModalClearsTerminalImage(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Preview.Protocol = LivePreviewKitty
	m.Preview.PNG = []byte{1, 2, 3}
	m.Width = minTerminalWidth
	m.Height = minTerminalHeight
	m.reflow()
	m.HelpOpen = true

	cmd := m.livePreviewModalCmd(false)
	if cmd == nil {
		t.Fatal("expected clear command when modal opens")
	}
}

func TestOverwriteConfirmationClearsAndRestoresLivePreview(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Preview.Protocol = LivePreviewKitty
	m.Preview.PNG = []byte{1, 2, 3}
	m.Width, m.Height = minTerminalWidth, minTerminalHeight
	m.DesignPresets = []config.DesignPreset{{Name: "existing", Document: m.Document}}
	m.Prompt = PromptState{Mode: PromptSaveDesign, Value: "existing"}

	m, cmd := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Prompt.Mode != PromptOverwriteDesign || cmd == nil {
		t.Fatalf("confirmation did not clear preview: mode=%q command=%v", m.Prompt.Mode, cmd)
	}
	m, cmd = m.update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Prompt.Mode != PromptNone || cmd == nil || m.Preview.RedrawSeq == 0 {
		t.Fatalf("cancel did not restore preview: mode=%q command=%v redraw=%d", m.Prompt.Mode, cmd, m.Preview.RedrawSeq)
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
