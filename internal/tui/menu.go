package tui

import tea "github.com/charmbracelet/bubbletea"

const menuItemCount = 2

func (m *Model) toggleMenu() bool {
	if m.MenuOpen {
		m.closeMenu("Menu closed.")
		return true
	}
	m.MenuOpen = true
	m.HelpOpen = false
	m.FocusPickerOpen = false
	m.closeFontPicker()
	m.MenuIndex = clampInt(m.MenuIndex, 0, menuItemCount-1)
	m.setStatus("Menu opened. Use ↑/↓ or k/j to move, Enter to select, 1 or 2 to leave.")
	return true
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
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
	if status != "" {
		m.setStatus("%s", status)
	}
}

func wrapMenuIndex(index int) int {
	index %= menuItemCount
	if index < 0 {
		index += menuItemCount
	}
	return index
}

func (m *Model) activateMenuItem() tea.Cmd {
	if m.MenuIndex == 1 {
		return m.toggleLivePreview()
	}
	m.AutoInsert = !m.AutoInsert
	if m.AutoInsert {
		m.setStatus("Auto Insert enabled.")
	} else {
		m.setStatus("Auto Insert disabled.")
	}
	return nil
}

// toggleLivePreview turns the live preview (inline image or native Preview window) off and back on.
func (m *Model) toggleLivePreview() tea.Cmd {
	if m.Preview.Protocol != LivePreviewDisabled {
		m.Preview.Protocol = LivePreviewDisabled
		m.Preview.PNG = nil
		m.Preview.PNGHash = ""
		m.Preview.RedrawPending = false
		openPreviewViewer.close()
		m.setStatus("Live preview disabled.")
		return nil
	}
	if m.Preview.Available == LivePreviewDisabled {
		m.setStatus("Live preview isn't available in this terminal.")
		return nil
	}
	m.Preview.Protocol = m.Preview.Available
	m.Preview.RequestedSeq++
	m.Preview.LastKey = m.livePreviewKey()
	m.setStatus("Live preview enabled.")
	return livePreviewDebounceCmd(m.Preview.RequestedSeq)
}
