package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/label"
)

const focusHintAlphabet = "asdfghjklqwertyuiopzxcvbnm1234567890"

func (m *Model) openFocusPicker() bool {
	if len(m.Document.Elements) == 0 {
		m.setStatus("No elements to focus.")
		return true
	}
	m.FocusPickerOpen = true
	m.HelpOpen = false
	m.FontPickerOpen = false
	m.FontPickerSearch = false
	m.FontPickerQuery = ""
	m.setStatus("Focus mode: press a hint key, or esc to cancel.")
	return true
}

func (m *Model) handleFocusPickerKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		m.FocusPickerOpen = false
		m.setStatus("Focus cancelled.")
		return true
	}
	if len(msg.Runes) == 0 {
		return true
	}
	hint := strings.ToLower(string(msg.Runes[0]))
	for _, candidate := range m.focusHints() {
		if candidate.Hint != hint {
			continue
		}
		m.SelectedID = candidate.Element.ID
		m.FocusPickerOpen = false
		m.Drag = DragState{}
		m.setStatus("Focused %q.", selectedLabel(candidate.Element))
		return true
	}
	m.setStatus("No focus target for %q.", hint)
	return true
}

type focusHint struct {
	Hint    string
	Element label.Element
}

func (m Model) focusHints() []focusHint {
	hints := make([]focusHint, 0, min(len(m.Document.Elements), len(focusHintAlphabet)))
	for i, element := range m.Document.Elements {
		if i >= len(focusHintAlphabet) {
			break
		}
		hints = append(hints, focusHint{Hint: string(focusHintAlphabet[i]), Element: element})
	}
	return hints
}
