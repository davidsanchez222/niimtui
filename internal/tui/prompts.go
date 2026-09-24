package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/render"
)

func (m *Model) handlePromptKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		m.Prompt = PromptState{}
		m.setStatus("Prompt cancelled.")
		return true
	case "enter":
		mode := m.Prompt.Mode
		value := m.Prompt.Value
		m.Prompt = PromptState{}
		switch mode {
		case PromptExportPNG:
			return m.exportPNGToPath(value)
		case PromptSaveDesign:
			return m.saveDesignPreset(value)
		}
		return true
	case "backspace":
		if m.Prompt.Value != "" {
			runes := []rune(m.Prompt.Value)
			m.Prompt.Value = string(runes[:len(runes)-1])
		}
		m.refreshPromptStatus()
		return true
	case "ctrl+u":
		m.Prompt.Value = ""
		m.refreshPromptStatus()
		return true
	}
	if len(msg.Runes) == 0 {
		return true
	}
	m.Prompt.Value += string(msg.Runes)
	m.refreshPromptStatus()
	return true
}

func (m *Model) refreshPromptStatus() {
	switch m.Prompt.Mode {
	case PromptExportPNG:
		m.setStatus("Export PNG path: %s%s (enter save, esc cancel)", m.Prompt.Value, string(promptCursorRune))
	case PromptSaveDesign:
		m.setStatus("Save design preset name: %s%s (enter save, esc cancel)", m.Prompt.Value, string(promptCursorRune))
	}
}

func (m *Model) beginExportPNGPrompt() bool {
	m.Prompt = PromptState{Mode: PromptExportPNG, Value: defaultExportPNGPath()}
	m.refreshPromptStatus()
	return true
}

func defaultExportPNGPath() string {
	return "./niimtui-" + time.Now().Format("20060102-150405") + ".png"
}

func (m *Model) exportPNGToPath(path string) bool {
	path = cleanExportPath(path)
	if path == "" {
		m.setStatus("Export PNG path is required.")
		return true
	}
	result, err := render.RenderDocument(m.Document)
	if err != nil {
		m.setStatus("Export render failed: %v", err)
		return true
	}
	result, err = m.preparePrintPreview(result)
	if err != nil {
		m.setStatus("Export print fitting failed: %v", err)
		return true
	}
	if err := writePNG(path, result.PreviewPNG); err != nil {
		m.setStatus("Export PNG failed: %v", err)
		return true
	}
	m.setStatus("Exported PNG to %s (%dx%d).", path, result.WidthPx, result.HeightPx)
	return true
}

func writePNG(path string, png []byte) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create export directory: %w", err)
		}
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	return nil
}
