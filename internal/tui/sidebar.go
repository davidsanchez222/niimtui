package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type sidebarSection uint8

const (
	sidebarPrinters sidebarSection = iota
	sidebarRolls
)

func (m *Model) focusSidebar() {
	m.SidebarFocused = true
	m.SidebarSection = sidebarPrinters
	for i, printer := range m.Print.Printers {
		if printer.Name == m.Print.Printer {
			m.SidebarPrinter = i
			break
		}
	}
	m.SidebarRoll = max(0, m.Preset)
	m.setStatus("Printer panel: arrows browse, enter selects, c connects, D disconnects, r rescans, esc returns.")
}

func (m *Model) handleSidebarKey(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc", "tab", "shift+tab":
		m.SidebarFocused = false
		m.setStatus("Label designer focused.")
	case "left", "right":
		if m.SidebarSection == sidebarPrinters {
			m.SidebarSection = sidebarRolls
		} else {
			m.SidebarSection = sidebarPrinters
		}
	case "up", "k", "down", "j":
		step := 1
		if key.String() == "up" || key.String() == "k" {
			step = -1
		}
		if m.SidebarSection == sidebarPrinters && len(m.Print.Printers) > 0 {
			m.SidebarPrinter = (m.SidebarPrinter + step + len(m.Print.Printers)) % len(m.Print.Printers)
		} else if m.SidebarSection == sidebarRolls && len(m.Presets) > 0 {
			m.SidebarRoll = (m.SidebarRoll + step + len(m.Presets)) % len(m.Presets)
		}
	case "enter":
		if m.SidebarSection == sidebarPrinters && m.SidebarPrinter < len(m.Print.Printers) {
			printer := m.Print.Printers[m.SidebarPrinter]
			if printer.Name != m.Print.Printer {
				return m.applyPrinter(printer)
			}
			return m.reconnectPrinter()
		}
		if m.SidebarSection == sidebarRolls && m.SidebarRoll < len(m.Presets) {
			m.applyPreset(m.SidebarRoll)
			m.commitHistory("change label roll")
		}
	case "c":
		if m.SidebarSection == sidebarPrinters && m.SidebarPrinter < len(m.Print.Printers) {
			printer := m.Print.Printers[m.SidebarPrinter]
			if printer.Name != m.Print.Printer {
				return m.applyPrinter(printer)
			}
		}
		return m.reconnectPrinter()
	case "D":
		m.disconnectPrinter()
	case "r":
		return m.startPrinterScan()
	}
	return nil
}

func sidebarWindow(length, selected, limit int) (int, int) {
	if length <= limit {
		return 0, length
	}
	start := min(max(0, selected-limit/2), length-limit)
	return start, start + limit
}

func sidebarLines(m Model, width int) []string {
	lines := []string{
		propertyTitleStyle.Render("Printer controls"),
		mutedStyle.Render("tab/esc editor · ←/→ section"),
		"",
		propertyLabel("Printers"),
	}
	start, end := sidebarWindow(len(m.Print.Printers), m.SidebarPrinter, 5)
	for i := start; i < end; i++ {
		printer := m.Print.Printers[i]
		prefix := "  "
		if m.SidebarSection == sidebarPrinters && i == m.SidebarPrinter {
			prefix = "> "
		}
		active := ""
		if printer.Name == m.Print.Printer {
			active = " *"
		}
		if m.DetectedNames != nil {
			if m.DetectedNames[printer.Name] {
				active += " seen"
			} else {
				active += " unseen"
			}
		}
		lines = append(lines, truncateText(prefix+printer.Name+active, width))
	}
	if len(m.Print.Printers) == 0 {
		lines = append(lines, mutedStyle.Render("None configured"))
	}
	lines = append(lines, "", propertyLabel("Installed rolls"))
	start, end = sidebarWindow(len(m.Presets), m.SidebarRoll, 5)
	for i := start; i < end; i++ {
		prefix := "  "
		if m.SidebarSection == sidebarRolls && i == m.SidebarRoll {
			prefix = "> "
		}
		active := ""
		if i == m.Preset {
			active = " *"
		}
		lines = append(lines, truncateText(prefix+m.Presets[i].Name+active, width))
	}
	if len(m.Presets) == 0 {
		lines = append(lines, mutedStyle.Render("No installed rolls"))
	}
	lines = append(lines, "", connectionStatusLine(m), connectHelp(m))
	if m.Discovering {
		lines = append(lines, mutedStyle.Render("Scanning for printers..."))
	}
	lines = append(lines, helpItem("r", "rescan"), mutedStyle.Render("↑/↓ browse · enter select"))
	return padLines(lines, width)
}

func sidebarFooter(m Model) string {
	if !m.SidebarFocused {
		return ""
	}
	return strings.Join([]string{helpItem("←/→", "section"), helpItem("↑/↓", "browse"), helpItem("enter", "select"), helpItem("tab", "editor")}, "  ")
}
