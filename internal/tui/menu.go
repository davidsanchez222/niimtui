package tui

import tea "github.com/charmbracelet/bubbletea"

const menuItemCount = 3

func (m *Model) toggleMenu() bool {
	m.MenuOpen = !m.MenuOpen
	if m.MenuOpen {
		m.HelpOpen = false
		m.FocusPickerOpen = false
		m.closeFontPicker()
		m.MenuIndex = clampInt(m.MenuIndex, 0, menuItemCount-1)
		m.setStatus("Menu opened. Use j/k or arrows, enter to select, esc to close.")
		return true
	}
	m.setStatus("Menu closed.")
	return true
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "m":
		m.MenuOpen = false
		m.setStatus("Menu closed.")
	case "up", "k":
		m.MenuIndex = wrapMenuIndex(m.MenuIndex - 1)
	case "down", "j":
		m.MenuIndex = wrapMenuIndex(m.MenuIndex + 1)
	case "enter":
		m.activateMenuItem()
	}
	return true
}

func wrapMenuIndex(index int) int {
	index %= menuItemCount
	if index < 0 {
		index += menuItemCount
	}
	return index
}

func (m *Model) activateMenuItem() {
	switch m.MenuIndex {
	case 0:
		m.AutoInsert = !m.AutoInsert
		if m.AutoInsert {
			m.setStatus("Auto Insert enabled.")
			return
		}
		m.setStatus("Auto Insert disabled.")
	case 1:
		m.setStatus("Edit Config is coming soon.")
	case 2:
		m.MenuOpen = false
		m.setStatus("Menu closed.")
	}
}
