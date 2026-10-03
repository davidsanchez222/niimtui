package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
)

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
		cmd := m.cycleLivePreviewMode()
		m.persistPreferences()
		return cmd
	}
	m.AutoInsert = !m.AutoInsert
	if m.AutoInsert {
		m.setStatus("Auto Insert enabled.")
	} else {
		m.setStatus("Auto Insert disabled.")
	}
	m.persistPreferences()
	return nil
}

// livePreviewModes is the menu cycle order. The Preview window mode is skipped where it can't run.
func livePreviewModes() []string {
	modes := []string{config.LivePreviewAuto, config.LivePreviewTerminal}
	if openPreviewAvailable() {
		modes = append(modes, config.LivePreviewWindow)
	}
	return append(modes, config.LivePreviewOff)
}

func nextLivePreviewMode(current string) string {
	if current == "" {
		current = config.LivePreviewAuto
	}
	modes := livePreviewModes()
	for i, mode := range modes {
		if mode == current {
			return modes[(i+1)%len(modes)]
		}
	}
	return modes[0]
}

// cycleLivePreviewMode advances the live preview mode and applies it. While the menu is open the
// terminal image is already cleared, so switching protocols only needs to drop the old render.
func (m *Model) cycleLivePreviewMode() tea.Cmd {
	m.LivePreviewMode = nextLivePreviewMode(m.LivePreviewMode)
	protocol := detectLivePreviewProtocol(m.LivePreviewMode)
	m.Preview.Protocol = LivePreviewDisabled
	m.Preview.PNG = nil
	m.Preview.PNGHash = ""
	m.Preview.RedrawPending = false
	openPreviewViewer.close()
	if protocol == LivePreviewDisabled {
		m.setStatus("Live preview: %s.", livePreviewMenuState(*m))
		return nil
	}
	m.Preview.Protocol = protocol
	m.Preview.RequestedSeq++
	m.Preview.LastKey = m.livePreviewKey()
	m.setStatus("Live preview: %s.", livePreviewMenuState(*m))
	return livePreviewDebounceCmd(m.Preview.RequestedSeq)
}
