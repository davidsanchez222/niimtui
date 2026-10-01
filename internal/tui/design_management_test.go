package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
	"niimtui/internal/label"
)

func testKey(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func TestOverwriteSavedDesignRequiresConfirmation(t *testing.T) {
	path := writeDesignFeatureConfig(t)
	m := NewModel(50, 30, "rect", "", PrintConfig{ConfigPath: path})
	m.saveDesignPreset("box")
	m.addTextElement()
	m.beginSaveDesignPrompt()
	m.Prompt.Value = "box"
	m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Prompt.Mode != PromptOverwriteDesign || len(m.DesignPresets[0].Document.Elements) != 0 {
		t.Fatalf("confirmation = %q, saved design = %#v", m.Prompt.Mode, m.DesignPresets[0])
	}
	m.Ready, m.Width, m.Height = true, minTerminalWidth, minTerminalHeight
	if view := m.View(); !strings.Contains(view, "Overwrite saved preset?") || !strings.Contains(view, `"box"`) {
		t.Fatalf("overwrite alert missing from view: %s", view)
	}
	m.handlePromptKey(testKey("n"))
	if m.Prompt.Mode != PromptNone || len(m.DesignPresets[0].Document.Elements) != 0 {
		t.Fatalf("cancelled overwrite changed design: %#v", m.DesignPresets[0])
	}
	before, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.DesignPresets[0].Document.Elements) != 0 {
		t.Fatal("cancelled overwrite changed config")
	}
	m.saveDesignPreset("box")
	m.handlePromptKey(testKey("y"))
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Prompt.Mode != PromptNone || len(loaded.DesignPresets) != 1 || len(loaded.DesignPresets[0].Document.Elements) != 1 {
		t.Fatalf("confirmed overwrite did not persist: %#v", loaded.DesignPresets)
	}
}

func TestOverwritePromptCanBeNavigatedAndDefaultsToCancel(t *testing.T) {
	path := writeDesignFeatureConfig(t)
	m := NewModel(50, 30, "rect", "", PrintConfig{ConfigPath: path})
	m.saveDesignPreset("box")
	m.addTextElement()
	m.saveDesignPreset("box")
	if m.Prompt.Mode != PromptOverwriteDesign || m.Prompt.Choice != 0 {
		t.Fatalf("unsafe confirmation default: %#v", m.Prompt)
	}
	m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.DesignPresets[0].Document.Elements) != 0 {
		t.Fatal("Enter overwrote without choosing Yes")
	}
	m.saveDesignPreset("box")
	m.handlePromptKey(testKey("j"))
	if m.Prompt.Choice != 1 {
		t.Fatalf("j did not select Overwrite: %#v", m.Prompt)
	}
	m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.DesignPresets[0].Document.Elements) != 1 {
		t.Fatal("navigated overwrite was not persisted")
	}
	m.saveDesignPreset("box")
	m.handlePromptKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.Prompt.Choice != 1 {
		t.Fatal("up arrow did not cycle confirmation options")
	}
}

func TestDeleteSavedPresetFromGalleryRequiresConfirmation(t *testing.T) {
	path := writeDesignFeatureConfig(t)
	m := NewModel(50, 30, "rect", "", PrintConfig{ConfigPath: path})
	m.saveDesignPreset("first")
	m.Document = label.NewDocument(40, 12)
	m.saveDesignPreset("second")
	m.loadDesignPreset(1)
	m.Tab = tabGallery
	for i, row := range m.galleryRows() {
		if !row.Header && row.Item.Name == "first" {
			m.GalleryIndex = i
			break
		}
	}
	m.handleGalleryKey(testKey("d"))
	if m.Prompt.Mode != PromptDeleteDesign || m.Prompt.Value != "first" {
		t.Fatalf("delete prompt = %#v", m.Prompt)
	}
	m.Ready, m.Width, m.Height = true, minTerminalWidth, minTerminalHeight
	if view := m.View(); !strings.Contains(view, "Delete saved preset?") || !strings.Contains(view, `"first"`) {
		t.Fatalf("delete alert missing from view: %s", view)
	}
	m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.DesignPresets) != 2 || m.Tab != tabGallery {
		t.Fatalf("cancel changed list or gallery: %#v", m.DesignPresets)
	}
	m.handleGalleryKey(testKey("d"))
	m.handlePromptKey(testKey("y"))
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.DesignPresets) != 1 || loaded.DesignPresets[0].Name != "second" || m.DesignPreset != 0 || m.Document.WidthMM != 40 {
		t.Fatalf("delete changed selected design or canvas: %#v, index=%d, doc=%#v", loaded.DesignPresets, m.DesignPreset, m.Document)
	}
	m.handleGalleryKey(testKey("d"))
	m.handlePromptKey(testKey("y"))
	loaded, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.DesignPresets) != 0 || m.DesignPreset != -1 || m.Tab != tabGallery || m.Document.WidthMM != 40 {
		t.Fatalf("last delete state: %#v, index=%d, tab=%d, doc=%#v", loaded.DesignPresets, m.DesignPreset, m.Tab, m.Document)
	}
}

func TestFailedPresetWriteLeavesCurrentListIntact(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{ConfigPath: filepath.Join(t.TempDir(), "missing", "config.json")})
	m.DesignPresets = []config.DesignPreset{{Name: "saved", Document: m.Document}}
	m.Document = label.NewDocument(40, 12)
	m.persistDesignPreset("saved")
	if !strings.Contains(m.Status, "failed") || len(m.DesignPresets) != 1 || m.DesignPresets[0].Document.WidthMM != 50 {
		t.Fatalf("failed save state: status=%q presets=%#v", m.Status, m.DesignPresets)
	}
	m.deleteDesignPreset("saved")
	if !strings.Contains(m.Status, "failed") || len(m.DesignPresets) != 1 {
		t.Fatalf("failed delete state: status=%q presets=%#v", m.Status, m.DesignPresets)
	}
}
