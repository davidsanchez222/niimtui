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
		cloned[i] = config.DesignPreset{Name: preset.Name, Document: cloneDocument(preset.Document), Bindings: append([]config.DesignBinding(nil), preset.Bindings...)}
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
	for _, preset := range m.DesignPresets {
		if preset.Name == name {
			m.Prompt = PromptState{Mode: PromptOverwriteDesign, Value: name}
			m.refreshPromptStatus()
			return true
		}
	}
	return m.persistDesignPreset(name)
}

func (m *Model) persistDesignPreset(name string) bool {
	preset := config.DesignPreset{Name: name, Document: cloneDocument(m.Document), Bindings: append([]config.DesignBinding(nil), m.Bindings...)}
	updated := append([]config.DesignPreset(nil), m.DesignPresets...)
	index := len(updated)
	for i := range updated {
		if updated[i].Name == name {
			updated[i] = preset
			index = i
			break
		}
	}
	if index == len(updated) {
		updated = append(updated, preset)
	}
	if err := saveDesignPresets(m.Print.ConfigPath, updated); err != nil {
		m.setStatus("Save design preset failed: %v", err)
		return true
	}
	m.DesignPresets = updated
	m.DesignPreset = index
	m.cleanDocument = cloneDocument(m.Document)
	m.cleanBindings = append([]config.DesignBinding(nil), m.Bindings...)
	m.unsavedTemplate = false
	m.setStatus("Saved design preset %q.", name)
	return true
}

func (m *Model) deleteDesignPreset(name string) bool {
	index := -1
	for i, preset := range m.DesignPresets {
		if preset.Name == name {
			index = i
			break
		}
	}
	if index < 0 {
		m.setStatus("Saved preset %q no longer exists.", name)
		return true
	}
	updated := append([]config.DesignPreset(nil), m.DesignPresets[:index]...)
	updated = append(updated, m.DesignPresets[index+1:]...)
	if err := saveDesignPresets(m.Print.ConfigPath, updated); err != nil {
		m.setStatus("Delete saved preset failed: %v", err)
		return true
	}
	m.DesignPresets = updated
	if m.DesignPreset == index {
		m.DesignPreset = -1
		m.unsavedTemplate = true
	} else if m.DesignPreset > index {
		m.DesignPreset--
	}
	if len(updated) == 0 {
		m.closeMenu("")
	} else {
		m.MenuListIndex = min(m.MenuListIndex, len(updated)-1)
	}
	m.setStatus("Deleted saved preset %q. Current canvas unchanged.", name)
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
	m.Bindings = append([]config.DesignBinding(nil), preset.Bindings...)
	m.cleanDocument = cloneDocument(m.Document)
	m.cleanBindings = append([]config.DesignBinding(nil), m.Bindings...)
	m.unsavedTemplate = false
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

func (m *Model) beginBindingPrompt() bool {
	element, ok := m.selectedElement()
	if !ok || (element.Text == nil && element.QR == nil) {
		m.setStatus("Select a text or QR element to bind.")
		return true
	}
	name := ""
	for _, binding := range m.Bindings {
		if binding.ElementID == element.ID {
			name = binding.Name
			break
		}
	}
	m.Prompt = PromptState{Mode: PromptBinding, Value: name}
	m.refreshPromptStatus()
	return true
}

func (m *Model) setSelectedBinding(name string) bool {
	element, ok := m.selectedElement()
	if !ok {
		return true
	}
	name = strings.TrimSpace(name)
	if name != "" && !config.ValidBindingName(name) {
		m.setStatus("Binding names must start with a letter and use letters, digits, _ or -.")
		return true
	}
	for _, binding := range m.Bindings {
		if binding.Name == name && binding.ElementID != element.ID {
			m.setStatus("Binding %q already belongs to another element.", name)
			return true
		}
	}
	for i, binding := range m.Bindings {
		if binding.ElementID == element.ID {
			if name == "" {
				m.Bindings = append(m.Bindings[:i], m.Bindings[i+1:]...)
				m.setStatus("Binding removed. Save the design to persist.")
			} else {
				m.Bindings[i].Name = name
				m.setStatus("Binding %q updated. Save the design to persist.", name)
			}
			m.commitHistory("change binding")
			return true
		}
	}
	if name != "" {
		m.Bindings = append(m.Bindings, config.DesignBinding{Name: name, ElementID: element.ID})
		m.setStatus("Binding %q added. Save the design to persist.", name)
		m.commitHistory("add binding")
	}
	return true
}

func (m *Model) toggleSelectedBindingRequired() bool {
	for i, binding := range m.Bindings {
		if binding.ElementID == m.SelectedID {
			m.Bindings[i].Required = !binding.Required
			m.setStatus("Binding %q required: %t. Save the design to persist.", binding.Name, m.Bindings[i].Required)
			m.commitHistory("toggle required binding")
			return true
		}
	}
	m.setStatus("Bind a selected element first (b).")
	return true
}

func (m *Model) removeBinding(elementID string) {
	for i, binding := range m.Bindings {
		if binding.ElementID == elementID {
			m.Bindings = append(m.Bindings[:i], m.Bindings[i+1:]...)
			return
		}
	}
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
