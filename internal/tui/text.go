package tui

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/label"
	"niimtui/internal/render"
)

const (
	defaultTextFontSize = 18.0
	minFontSize         = 8.0
	maxFontSize         = 200.0
)

func (m *Model) addTextElement() {
	id := fmt.Sprintf("text-%d", m.NextID)
	m.NextID++

	element := label.NewTextElement(
		id,
		"",
		math.Max(m.Document.WidthMM*0.1, 1),
		math.Max(m.Document.HeightMM*0.1, 1),
		math.Max(math.Min(m.Document.WidthMM*0.4, m.Document.WidthMM-2), 10),
		math.Max(math.Min(m.Document.HeightMM*0.2, m.Document.HeightMM-2), 6),
		defaultTextFontSize,
	)
	element.Text.FontPath = m.FontPath
	clampElementToDocument(&element, m.Document)
	if err := m.Document.AddElement(element); err != nil {
		m.setStatus("Add text failed: %v", err)
		return
	}

	m.SelectedID = id
	m.TextBuffer = element.Text.Value
	m.EditingText = true
	m.refreshStatus()
}

func (m *Model) addQRElement() {
	id := fmt.Sprintf("qr-%d", m.NextID)
	m.NextID++

	sizeMM := math.Max(math.Min(m.Document.WidthMM, m.Document.HeightMM)*0.35, 8)
	element := label.NewQRElement(
		id,
		"https://google.com",
		math.Max((m.Document.WidthMM-sizeMM)/2, 0),
		math.Max((m.Document.HeightMM-sizeMM)/2, 0),
		sizeMM,
	)
	clampElementToDocument(&element, m.Document)
	if err := m.Document.AddElement(element); err != nil {
		m.setStatus("Add QR failed: %v", err)
		return
	}

	m.SelectedID = id
	m.TextBuffer = element.QR.Value
	m.EditingText = true
	m.refreshStatus()
}

func (m *Model) beginEditingSelected() bool {
	element, ok := m.selectedElement()
	if !ok || (element.Text == nil && element.QR == nil) {
		return false
	}
	m.EditingText = true
	m.TextBuffer = editableElementValue(element)
	m.refreshStatus()
	return true
}

func (m *Model) deleteSelected() bool {
	if m.SelectedID == "" {
		return false
	}
	deletedID := m.SelectedID
	if !m.Document.DeleteElement(deletedID) {
		return false
	}
	m.SelectedID = ""
	m.Drag = DragState{}
	m.EditingText = false
	m.FocusPickerOpen = false
	m.closeFontPicker()
	m.TextBuffer = ""
	m.setStatus("Deleted %q.", deletedID)
	return true
}

func (m *Model) toggleFontPicker() bool {
	element, ok := m.selectedElement()
	if !ok || element.Text == nil {
		return false
	}
	if m.FontPickerOpen {
		m.closeFontPicker()
		m.setStatus("Font picker closed.")
		return true
	}
	m.FontPickerOpen = true
	m.FontPickerSearch = true
	m.FontPickerQuery = ""
	m.FontPickerIndex = fontOptionIndex(m.Fonts, element.Text.FontPath)
	m.setStatus("Type to search fonts. Ctrl+n/p moves matches, esc browses results.")
	return true
}

func (m *Model) handleFontPickerKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		if m.FontPickerSearch {
			m.FontPickerSearch = false
			m.setStatus("Browsing font results. Use j/k or ctrl+d/u, enter to apply.")
			return true
		}
		m.closeFontPicker()
		m.setStatus("Font picker closed.")
		return true
	case "up", "ctrl+p":
		return m.moveFontPicker(-1)
	case "down", "ctrl+n":
		return m.moveFontPicker(1)
	case "k":
		if !m.FontPickerSearch {
			return m.moveFontPicker(-1)
		}
		if len(msg.Runes) > 0 {
			return m.appendFontSearch(string(msg.Runes))
		}
	case "j":
		if !m.FontPickerSearch {
			return m.moveFontPicker(1)
		}
		if len(msg.Runes) > 0 {
			return m.appendFontSearch(string(msg.Runes))
		}
	case "/":
		if !m.FontPickerSearch {
			m.FontPickerSearch = true
			m.setStatus("Font search resumed.")
			return true
		}
		if len(msg.Runes) > 0 {
			return m.appendFontSearch(string(msg.Runes))
		}
	case "ctrl+u":
		if !m.FontPickerSearch {
			return m.moveFontPicker(-fontPickerPageSize)
		}
		m.FontPickerQuery = ""
		m.selectFirstFilteredFont()
		m.setStatus("Font search cleared.")
		return true
	case "ctrl+d":
		if !m.FontPickerSearch {
			return m.moveFontPicker(fontPickerPageSize)
		}
	case "enter":
		return m.applySelectedFont()
	case "backspace":
		if m.FontPickerSearch {
			return m.removeFontSearchRune()
		}
	default:
		if m.FontPickerSearch && len(msg.Runes) > 0 {
			return m.appendFontSearch(string(msg.Runes))
		}
	}
	return true
}

func (m *Model) closeFontPicker() {
	m.FontPickerOpen = false
	m.FontPickerSearch = false
	m.FontPickerQuery = ""
}

const fontPickerPageSize = 7

func (m *Model) moveFontPicker(delta int) bool {
	indices := m.filteredFontIndices()
	if len(indices) == 0 {
		m.setStatus("No fonts match %q.", m.FontPickerQuery)
		return true
	}
	position := 0
	for i, index := range indices {
		if index == m.FontPickerIndex {
			position = i
			break
		}
	}
	position = clampInt(position+delta, 0, len(indices)-1)
	m.FontPickerIndex = indices[position]
	m.setStatus("Font: %s", m.Fonts[m.FontPickerIndex].Name)
	return true
}

func (m *Model) appendFontSearch(s string) bool {
	m.FontPickerQuery += s
	if !m.selectFirstFilteredFont() {
		m.setStatus("No fonts match %q.", m.FontPickerQuery)
		return true
	}
	m.setStatus("Font search: %s", m.FontPickerQuery)
	return true
}

func (m *Model) removeFontSearchRune() bool {
	if m.FontPickerQuery == "" {
		return true
	}
	runes := []rune(m.FontPickerQuery)
	m.FontPickerQuery = string(runes[:len(runes)-1])
	m.selectFirstFilteredFont()
	if m.FontPickerQuery == "" {
		m.setStatus("Font search cleared.")
		return true
	}
	m.setStatus("Font search: %s", m.FontPickerQuery)
	return true
}

func (m *Model) selectFirstFilteredFont() bool {
	indices := m.filteredFontIndices()
	if len(indices) == 0 {
		return false
	}
	for _, index := range indices {
		if index == m.FontPickerIndex {
			return true
		}
	}
	m.FontPickerIndex = indices[0]
	return true
}

func (m Model) filteredFontIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.FontPickerQuery))
	indices := make([]int, 0, len(m.Fonts))
	for i, font := range m.Fonts {
		if query == "" || strings.Contains(strings.ToLower(font.Name), query) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *Model) applySelectedFont() bool {
	element, ok := m.selectedElement()
	indices := m.filteredFontIndices()
	if !ok || element.Text == nil {
		m.FontPickerOpen = false
		return false
	}
	if len(indices) == 0 {
		m.setStatus("No fonts match %q.", m.FontPickerQuery)
		return true
	}
	m.selectFirstFilteredFont()
	m.FontPickerIndex = clampInt(m.FontPickerIndex, 0, len(m.Fonts)-1)
	fontOption := m.Fonts[m.FontPickerIndex]
	if err := render.ValidateFontPath(fontOption.Path); err != nil {
		m.setStatus("Font invalid: %v", err)
		return true
	}
	element.Text.FontPath = fontOption.Path
	autoFitTextElement(&element)
	clampElementToDocument(&element, m.Document)
	if !m.Document.UpdateElement(element) {
		m.setStatus("Apply font failed.")
		return false
	}
	m.FontPath = fontOption.Path
	m.FontPickerOpen = false
	m.FontPickerSearch = false
	m.FontPickerQuery = ""
	m.setStatus("Font set to %s.", fontOption.Name)
	return true
}

func (m *Model) nudgeSelected(dxMM, dyMM float64) bool {
	element, ok := m.selectedElement()
	if !ok {
		return false
	}
	element.XMM += dxMM
	element.YMM += dyMM
	clampElementToDocument(&element, m.Document)
	if !m.Document.UpdateElement(element) {
		return false
	}
	m.setStatus("Moved to x %.1fmm y %.1fmm", element.XMM, element.YMM)
	return true
}

func (m *Model) resizeSelected(handle ResizeHandle, dxMM, dyMM float64) bool {
	element, ok := m.selectedElement()
	if !ok {
		return false
	}
	updated := resizeElement(element, handle, dxMM, dyMM)
	clampElementToDocument(&updated, m.Document)
	if !m.Document.UpdateElement(updated) {
		return false
	}
	m.setStatus("Resized to %.1fmm x %.1fmm", updated.WidthMM, updated.HeightMM)
	return true
}

func (m *Model) adjustSelectedFont(delta float64) bool {
	element, ok := m.selectedElement()
	if !ok || element.Text == nil {
		return false
	}
	fontSize := math.Max(minFontSize, math.Min(maxFontSize, element.Text.FontSize+delta))
	if fontSize == element.Text.FontSize {
		return true
	}
	element.Text.FontSize = fontSize
	autoFitTextElement(&element)
	if !m.Document.UpdateElement(element) {
		return false
	}
	m.setStatus("Font size %.0f", fontSize)
	return true
}

func (m *Model) handleTextEditing(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		m.EditingText = false
		m.TextBuffer = ""
		m.setStatus("Exited text editing.")
		return
	case "enter":
		m.finishTextEdit()
		return
	case "backspace":
		if len(m.TextBuffer) == 0 {
			_ = m.applyTextBuffer("")
			m.refreshStatus()
			return
		}
		runes := []rune(m.TextBuffer)
		m.TextBuffer = string(runes[:len(runes)-1])
		_ = m.applyTextBuffer(m.TextBuffer)
		m.refreshStatus()
		return
	}

	if len(msg.Runes) == 0 {
		return
	}
	m.TextBuffer += string(msg.Runes)

	_ = m.applyTextBuffer(m.TextBuffer)
	m.refreshStatus()
}

func (m *Model) finishTextEdit() {
	m.EditingText = false
	m.TextBuffer = ""
	m.setStatus("Exited text editing.")
}

func (m *Model) applyTextBuffer(value string) bool {
	element, ok := m.selectedElement()
	if !ok || (element.Text == nil && element.QR == nil) {
		m.EditingText = false
		m.TextBuffer = ""
		m.setStatus("No editable element selected.")
		return false
	}
	if element.Text != nil {
		element.Text.Value = value
		autoFitTextElement(&element)
	}
	if element.QR != nil {
		element.QR.Value = value
	}
	if !m.Document.UpdateElement(element) {
		m.setStatus("Save edit failed.")
		return false
	}
	return true
}

func editableElementValue(element label.Element) string {
	if element.Text != nil {
		return element.Text.Value
	}
	if element.QR != nil {
		return element.QR.Value
	}
	return ""
}

func autoFitTextElement(element *label.Element) {
	if element == nil || element.Text == nil {
		return
	}
	_, minHeightMM := minimumElementSize(*element)
	if element.HeightMM < minHeightMM {
		element.HeightMM = minHeightMM
	}
	minWidthMM, _ := minimumElementSize(*element)
	if element.WidthMM < minWidthMM {
		element.WidthMM = minWidthMM
	}
	if requiredHeightMM, err := render.RequiredTextHeightMM(*element, element.WidthMM); err == nil && requiredHeightMM > element.HeightMM {
		element.HeightMM = requiredHeightMM
	}
}
