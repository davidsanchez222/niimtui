package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/render"
)

func (m Model) confirmPromptOpen() bool {
	switch m.Prompt.Mode {
	case PromptOverwriteDesign, PromptDeleteDesign, PromptOpenGallery, PromptCommand:
		return true
	}
	return false
}

func (m *Model) handlePromptKey(msg tea.KeyMsg) bool {
	if m.confirmPromptOpen() {
		mode, name := m.Prompt.Mode, m.Prompt.Value
		if mode == PromptCommand {
			switch msg.String() {
			case "esc", "enter":
				m.Prompt = PromptState{}
				m.setStatus("Closed print command.")
			case "c":
				if err := copyTerminalCommand(name); err != nil {
					m.setStatus("Could not copy command: %v", err)
				} else {
					m.setStatus("Sent print command to terminal clipboard (if supported).")
				}
			case "up", "k":
				m.CommandScroll = max(0, m.CommandScroll-1)
			case "down", "j":
				m.CommandScroll++
			}
			return true
		}
		switch msg.String() {
		case "y":
			m.Prompt = PromptState{}
			if mode == PromptOverwriteDesign {
				return m.persistDesignPreset(name)
			}
			if mode == PromptOpenGallery {
				item, ok := m.currentGalleryItem()
				if ok && item.Name == name {
					_ = m.loadGalleryItem(item)
				}
				return true
			}
			return m.deleteDesignPreset(name)
		case "n", "esc":
			m.Prompt = PromptState{}
			m.setStatus("Cancelled change to saved preset %q.", name)
		}
		return true
	}
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
		case PromptBinding:
			return m.setSelectedBinding(value)
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

func copyTerminalCommand(command string) error {
	_, err := fmt.Fprintf(os.Stdout, "\x1b]52;c;%s\a", base64.StdEncoding.EncodeToString([]byte(command)))
	return err
}

func (m *Model) refreshPromptStatus() {
	switch m.Prompt.Mode {
	case PromptExportPNG:
		m.setStatus("Export PNG path: %s%s (enter save, esc cancel)", m.Prompt.Value, string(promptCursorRune))
	case PromptSaveDesign:
		m.setStatus("Save design preset name: %s%s (enter save, esc cancel)", m.Prompt.Value, string(promptCursorRune))
	case PromptBinding:
		m.setStatus("Binding name: %s%s (enter set, empty removes, esc cancel)", m.Prompt.Value, string(promptCursorRune))
	case PromptOverwriteDesign:
		m.setStatus("Saved preset %q already exists. Overwrite? y yes / n or esc cancel", m.Prompt.Value)
	case PromptDeleteDesign:
		m.setStatus("Delete saved preset %q? y yes / n or esc cancel", m.Prompt.Value)
	case PromptOpenGallery:
		m.setStatus("Discard unsaved changes and open %q? y yes / n or esc cancel", m.Prompt.Value)
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
