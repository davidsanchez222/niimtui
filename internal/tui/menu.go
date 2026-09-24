package tui

import tea "github.com/charmbracelet/bubbletea"

const menuItemCount = 6

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

func (m *Model) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "m":
		m.MenuOpen = false
		m.setStatus("Menu closed.")
	case "up", "k":
		m.MenuIndex = wrapMenuIndex(m.MenuIndex - 1)
	case "down", "j":
		m.MenuIndex = wrapMenuIndex(m.MenuIndex + 1)
	case "enter":
		return m.activateMenuItem()
	}
	return nil
}

func wrapMenuIndex(index int) int {
	index %= menuItemCount
	if index < 0 {
		index += menuItemCount
	}
	return index
}

func (m *Model) activateMenuItem() tea.Cmd {
	switch m.MenuIndex {
	case 0:
		return m.switchPrinter(1)
	case 1:
		m.switchPreset(1)
	case 2:
		m.loadNextDesignPreset()
	case 3:
		m.beginSaveDesignPrompt()
		m.MenuOpen = false
	case 4:
		m.AutoInsert = !m.AutoInsert
		if m.AutoInsert {
			m.setStatus("Auto Insert enabled.")
			return nil
		}
		m.setStatus("Auto Insert disabled.")
	case 5:
		m.MenuOpen = false
		m.setStatus("Menu closed.")
	}
	return nil
}
