package tui

import (
	"fmt"
	"strings"

	"niimtui/internal/label"
)

const pasteOffsetMM = 2.0

func (m *Model) copySelected() bool {
	element, ok := m.selectedElement()
	if !ok {
		return false
	}
	m.Clipboard = []label.Element{cloneElement(element)}
	m.setStatus("Copied %s.", element.ID)
	return true
}

func (m *Model) cutSelected() bool {
	element, ok := m.selectedElement()
	if !ok {
		return false
	}
	m.Clipboard = []label.Element{cloneElement(element)}
	if !m.Document.DeleteElement(element.ID) {
		return false
	}
	m.SelectedID = ""
	m.Drag = DragState{}
	m.EditingText = false
	m.TextBuffer = ""
	m.commitHistory("cut component")
	m.setStatus("Cut %s.", element.ID)
	return true
}

func (m *Model) duplicateSelected() bool {
	element, ok := m.selectedElement()
	if !ok {
		return false
	}
	duplicated := m.preparePastedElement(element, pasteOffsetMM)
	if err := m.Document.AddElement(duplicated); err != nil {
		m.setStatus("Duplicate failed: %v", err)
		return true
	}
	m.SelectedID = duplicated.ID
	m.TextBuffer = editableElementValue(duplicated)
	m.commitHistory("duplicate component")
	m.setStatus("Duplicated %s as %s.", element.ID, duplicated.ID)
	return true
}

func (m *Model) pasteClipboard() bool {
	if len(m.Clipboard) == 0 {
		m.setStatus("Clipboard is empty.")
		return true
	}
	var pastedIDs []string
	for i, element := range m.Clipboard {
		pasted := m.preparePastedElement(element, pasteOffsetMM*float64(i+1))
		if err := m.Document.AddElement(pasted); err != nil {
			m.setStatus("Paste failed: %v", err)
			return true
		}
		pastedIDs = append(pastedIDs, pasted.ID)
		m.SelectedID = pasted.ID
		m.TextBuffer = editableElementValue(pasted)
	}
	m.commitHistory("paste component")
	m.setStatus("Pasted %s.", strings.Join(pastedIDs, ", "))
	return true
}

func (m *Model) preparePastedElement(element label.Element, offsetMM float64) label.Element {
	pasted := cloneElement(element)
	pasted.ID = m.nextElementID(pasted.Type)
	pasted.XMM += offsetMM
	pasted.YMM += offsetMM
	clampElementToDocument(&pasted, m.Document)
	return pasted
}

func (m *Model) nextElementID(elementType label.ElementType) string {
	prefix := "element"
	switch elementType {
	case label.ElementText:
		prefix = "text"
	case label.ElementQR:
		prefix = "qr"
	}
	for {
		id := fmt.Sprintf("%s-%d", prefix, m.NextID)
		m.NextID++
		if _, exists := m.Document.ElementByID(id); !exists {
			return id
		}
	}
}
