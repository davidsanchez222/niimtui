package tui

import tea "github.com/charmbracelet/bubbletea"

const menuItemCount = 5

func (m *Model) toggleMenu() bool {
	if m.MenuOpen {
		m.closeMenu("Menu closed.")
		return true
	}
	m.MenuOpen = true
	m.MenuListMode = MenuListNone
	m.MenuListIndex = 0
	m.HelpOpen = false
	m.FocusPickerOpen = false
	m.closeFontPicker()
	m.MenuIndex = clampInt(m.MenuIndex, 0, menuItemCount-1)
	m.setStatus("Menu opened. Use j/k or arrows, enter opens/selects, esc closes.")
	return true
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
	if m.MenuListMode != MenuListNone {
		return m.handleMenuListKey(msg)
	}
	switch msg.String() {
	case "esc", "m":
		m.closeMenu("Menu closed.")
	case "up", "k":
		m.MenuIndex = wrapMenuIndex(m.MenuIndex - 1)
	case "down", "j":
		m.MenuIndex = wrapMenuIndex(m.MenuIndex + 1)
	case "enter":
		return m.activateMenuItem()
	}
	return nil
}

func (m *Model) closeMenu(status string) {
	m.MenuOpen = false
	m.MenuListMode = MenuListNone
	m.MenuListIndex = 0
	if status != "" {
		m.setStatus("%s", status)
	}
}

func (m *Model) handleMenuListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "m":
		m.closeMenu("Menu closed.")
	case "up", "k":
		m.moveMenuList(-1)
	case "down", "j":
		m.moveMenuList(1)
	case "enter":
		m.activateMenuListSelection()
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
		m.openMenuList(MenuListLabelRolls)
	case 1:
		m.openMenuList(MenuListDesignPresets)
	case 2:
		m.beginSaveDesignPrompt()
		m.closeMenu("")
	case 3:
		m.AutoInsert = !m.AutoInsert
		if m.AutoInsert {
			m.setStatus("Auto Insert enabled.")
			return nil
		}
		m.setStatus("Auto Insert disabled.")
	case 4:
		m.closeMenu("Menu closed.")
	}
	return nil
}

func (m *Model) openMenuList(mode MenuListMode) bool {
	count := m.menuListCount(mode)
	if count == 0 {
		switch mode {
		case MenuListLabelRolls:
			m.setStatus("No label presets available.")
		case MenuListDesignPresets:
			m.setStatus("No saved design presets.")
		}
		return true
	}
	m.MenuListMode = mode
	m.MenuListIndex = clampInt(m.menuListActiveIndex(mode), 0, count-1)
	switch mode {
	case MenuListLabelRolls:
		m.setStatus("Select a label roll.")
	case MenuListDesignPresets:
		m.setStatus("Select a saved preset.")
	}
	return true
}

func (m Model) menuListCount(mode MenuListMode) int {
	switch mode {
	case MenuListLabelRolls:
		return len(m.Presets)
	case MenuListDesignPresets:
		return len(m.DesignPresets)
	default:
		return 0
	}
}

func (m Model) menuListActiveIndex(mode MenuListMode) int {
	switch mode {
	case MenuListLabelRolls:
		if m.Preset >= 0 && m.Preset < len(m.Presets) {
			return m.Preset
		}
		return activePresetIndex(m.Presets, "", m.Document)
	case MenuListDesignPresets:
		if m.DesignPreset >= 0 && m.DesignPreset < len(m.DesignPresets) {
			return m.DesignPreset
		}
	}
	return 0
}

func (m *Model) moveMenuList(delta int) bool {
	count := m.menuListCount(m.MenuListMode)
	if count == 0 || delta == 0 {
		return true
	}
	m.MenuListIndex = (m.MenuListIndex + delta) % count
	if m.MenuListIndex < 0 {
		m.MenuListIndex += count
	}
	return true
}

func (m *Model) activateMenuListSelection() bool {
	mode := m.MenuListMode
	index := m.MenuListIndex
	if index < 0 || index >= m.menuListCount(mode) {
		return true
	}
	m.closeMenu("")
	switch mode {
	case MenuListLabelRolls:
		m.applyPreset(index)
		m.commitHistory("change label roll")
	case MenuListDesignPresets:
		m.loadDesignPreset(index)
	}
	return true
}
