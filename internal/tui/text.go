package tui

import (
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"niimcli/internal/label"
)

const (
	defaultTextFontSize = 18.0
	minFontSize         = 8.0
	maxFontSize         = 72.0
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

func (m *Model) beginEditingSelected() bool {
	element, ok := m.selectedElement()
	if !ok || element.Text == nil {
		return false
	}
	m.EditingText = true
	m.TextBuffer = element.Text.Value
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
	m.TextBuffer = ""
	m.setStatus("Deleted %q.", deletedID)
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
	if !ok || element.Text == nil {
		m.EditingText = false
		m.TextBuffer = ""
		m.setStatus("No selected text element.")
		return false
	}
	element.Text.Value = value
	if !m.Document.UpdateElement(element) {
		m.setStatus("Save text failed.")
		return false
	}
	return true
}
