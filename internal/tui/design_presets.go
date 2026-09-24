package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"niimtui/internal/config"
	"niimtui/internal/label"
)

func cloneDesignPresets(presets []config.DesignPreset) []config.DesignPreset {
	cloned := make([]config.DesignPreset, len(presets))
	for i, preset := range presets {
		cloned[i] = config.DesignPreset{Name: preset.Name, Document: cloneDocument(preset.Document)}
	}
	return cloned
}

func (m *Model) beginSaveDesignPrompt() {
	m.Prompt = PromptState{Mode: PromptSaveDesign, Value: defaultDesignPresetName()}
	m.setStatus("Save design preset name: %s (enter save, esc cancel)", m.Prompt.Value)
}

func defaultDesignPresetName() string {
	return "design-" + time.Now().Format("20060102-150405")
}

func (m *Model) saveDesignPreset(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		m.setStatus("Design preset name is required.")
		return true
	}
	preset := config.DesignPreset{Name: name, Document: cloneDocument(m.Document)}
	updated := false
	for i := range m.DesignPresets {
		if m.DesignPresets[i].Name == name {
			m.DesignPresets[i] = preset
			m.DesignPreset = i
			updated = true
			break
		}
	}
	if !updated {
		m.DesignPresets = append(m.DesignPresets, preset)
		m.DesignPreset = len(m.DesignPresets) - 1
	}
	if err := saveDesignPresets(m.Print.ConfigPath, m.DesignPresets); err != nil {
		m.setStatus("Save design preset failed: %v", err)
		return true
	}
	m.setStatus("Saved design preset %q.", name)
	return true
}

func saveDesignPresets(path string, presets []config.DesignPreset) error {
	path = strings.TrimSpace(path)
	if path == "" {
		defaultPath, err := config.DefaultPath()
		if err != nil {
			return err
		}
		path = defaultPath
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cfg.DesignPresets = cloneDesignPresets(presets)
	return config.Save(path, cfg)
}

func (m *Model) loadNextDesignPreset() bool {
	if len(m.DesignPresets) == 0 {
		m.setStatus("No saved design presets.")
		return true
	}
	next := m.DesignPreset + 1
	if next < 0 || next >= len(m.DesignPresets) {
		next = 0
	}
	m.loadDesignPreset(next)
	return true
}

func (m *Model) loadDesignPreset(index int) {
	if index < 0 || index >= len(m.DesignPresets) {
		return
	}
	preset := m.DesignPresets[index]
	m.Document = cloneDocument(preset.Document)
	m.DesignPreset = index
	m.Preset = activePresetIndex(m.Presets, "", m.Document)
	m.NextID = nextIDForDocument(m.Document)
	m.SelectedID = ""
	m.Drag = DragState{}
	m.EditingText = false
	m.TextBuffer = ""
	m.reflow()
	m.commitHistory("load design preset")
	m.setStatus("Loaded design preset %q.", preset.Name)
}

func nextIDForDocument(doc label.Document) int {
	nextID := 1
	for _, element := range doc.Elements {
		separator := strings.LastIndex(element.ID, "-")
		if separator < 0 || separator == len(element.ID)-1 {
			continue
		}
		value, err := strconv.Atoi(element.ID[separator+1:])
		if err == nil && value >= nextID {
			nextID = value + 1
		}
	}
	return nextID
}

func (m Model) currentDesignPresetLabel() string {
	if m.DesignPreset >= 0 && m.DesignPreset < len(m.DesignPresets) {
		return m.DesignPresets[m.DesignPreset].Name
	}
	if len(m.DesignPresets) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d saved", len(m.DesignPresets))
}

func cleanExportPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}
