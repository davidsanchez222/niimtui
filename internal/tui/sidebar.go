package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
)

type sidebarRow struct {
	Printer config.PrinterProfile
	Roll    config.LabelPreset
	Header  bool
	Empty   bool
}

func (m Model) sidebarRows() []sidebarRow {
	rows := make([]sidebarRow, 0, len(m.Print.Printers)+len(m.AllPresets))
	for _, printer := range m.Print.Printers {
		rows = append(rows, sidebarRow{Printer: printer, Header: true})
		if m.SidebarCollapsed[printer.Name] {
			continue
		}
		rolls := presetsForPrinter(m.AllPresets, printer.Model)
		if len(rolls) == 0 {
			rows = append(rows, sidebarRow{Printer: printer, Empty: true})
		}
		for _, roll := range rolls {
			rows = append(rows, sidebarRow{Printer: printer, Roll: roll})
		}
	}
	return rows
}

func (m Model) currentSidebarRow() (sidebarRow, bool) {
	rows := m.sidebarRows()
	if m.SidebarIndex < 0 || m.SidebarIndex >= len(rows) {
		return sidebarRow{}, false
	}
	return rows[m.SidebarIndex], true
}

func (m *Model) focusSidebar() {
	m.SidebarFocused = true
	for i, row := range m.sidebarRows() {
		if row.Header && row.Printer.Name == m.Print.Printer {
			m.SidebarIndex = i
			break
		}
	}
	m.setStatus("Printer picker: ↑/↓ or k/j browse, enter switches printer/roll, ←/→ or h/l folds, esc returns.")
}

func (m *Model) foldSidebarPrinter(name string, collapsed bool) {
	if m.SidebarCollapsed[name] == collapsed {
		return
	}
	selected, _ := m.currentSidebarRow()
	if m.SidebarCollapsed == nil {
		m.SidebarCollapsed = make(map[string]bool)
	}
	m.SidebarCollapsed[name] = collapsed
	for i, row := range m.sidebarRows() {
		if row.Printer.Name == selected.Printer.Name &&
			((selected.Printer.Name == name && collapsed && row.Header) ||
				(row.Header == selected.Header && (row.Header || row.Roll.Name == selected.Roll.Name))) {
			m.SidebarIndex = i
			break
		}
	}
}

func (m *Model) selectSidebarRow(row sidebarRow) tea.Cmd {
	if row.Header {
		if row.Printer.Name == m.Print.Printer {
			return m.reconnectPrinter()
		}
		return m.applyPrinter(row.Printer)
	}
	var cmd tea.Cmd
	if row.Printer.Name != m.Print.Printer {
		cmd = m.applyPrinter(row.Printer)
	}
	for i, roll := range m.Presets {
		if roll.Name != row.Roll.Name {
			continue
		}
		m.applyPreset(i)
		m.commitHistory("change label roll")
		break
	}
	return cmd
}

func (m *Model) handleSidebarKey(key tea.KeyMsg) tea.Cmd {
	rows := m.sidebarRows()
	switch key.String() {
	case "esc", "tab", "shift+tab":
		m.SidebarFocused = false
		m.setStatus("Label designer focused.")
	case "up", "k", "down", "j":
		if len(rows) == 0 {
			return nil
		}
		step := 1
		if key.String() == "up" || key.String() == "k" {
			step = -1
		}
		for i := 0; i < len(rows); i++ {
			m.SidebarIndex = (m.SidebarIndex + step + len(rows)) % len(rows)
			if !rows[m.SidebarIndex].Empty {
				break
			}
		}
	case "left", "h", "right", "l", " ", "space":
		row, ok := m.currentSidebarRow()
		if !ok {
			return nil
		}
		collapsed := key.String() == "left" || key.String() == "h"
		if key.String() == " " || key.String() == "space" {
			collapsed = !m.SidebarCollapsed[row.Printer.Name]
		}
		m.foldSidebarPrinter(row.Printer.Name, collapsed)
	case "enter":
		if row, ok := m.currentSidebarRow(); ok && !row.Empty {
			return m.selectSidebarRow(row)
		}
	case "c":
		if row, ok := m.currentSidebarRow(); ok && !row.Empty {
			return m.selectSidebarRow(row)
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
	lines := []string{propertyTitleStyle.Render("Printer controls"), mutedStyle.Render("↑/↓ k/j browse · enter select")}
	rows := m.sidebarRows()
	start, end := sidebarWindow(len(rows), m.SidebarIndex, max(1, m.canvasPanelHeight()-8))
	for i := start; i < end; i++ {
		row := rows[i]
		prefix := "  "
		if i == m.SidebarIndex {
			prefix = "> "
		}
		if row.Header {
			arrow := "▾ "
			if m.SidebarCollapsed[row.Printer.Name] {
				arrow = "▸ "
			}
			active := ""
			if row.Printer.Name == m.Print.Printer {
				active = " *"
			}
			if m.DetectedNames != nil {
				if m.DetectedNames[row.Printer.Name] {
					active += " seen"
				} else {
					active += " unseen"
				}
			}
			lines = append(lines, truncateText(prefix+arrow+row.Printer.Name+active, width))
			continue
		}
		if row.Empty {
			lines = append(lines, mutedStyle.Render("    No installed rolls"))
			continue
		}
		active := ""
		if row.Printer.Name == m.Print.Printer && m.Preset >= 0 && m.Preset < len(m.Presets) && m.Presets[m.Preset].Name == row.Roll.Name {
			active = " *"
		}
		lines = append(lines, truncateText(prefix+"  "+row.Roll.Name+active, width))
	}
	if len(rows) == 0 {
		lines = append(lines, mutedStyle.Render("No printers configured"))
	}
	lines = append(lines, "", connectionStatusLine(m), connectHelp(m))
	if m.Discovering {
		lines = append(lines, mutedStyle.Render("Scanning for printers..."))
	}
	lines = append(lines, helpItem("r", "rescan"), mutedStyle.Render("←/→ h/l fold · space toggle"))
	return padLines(lines, width)
}

func sidebarFooter(m Model) string {
	if !m.SidebarFocused {
		return ""
	}
	return strings.Join([]string{helpItem("↑/↓ k/j", "browse"), helpItem("←/→ h/l", "fold"), helpItem("enter", "select"), helpItem("tab", "editor")}, "  ")
}
