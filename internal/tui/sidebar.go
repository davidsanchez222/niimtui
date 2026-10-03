package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"niimtui/internal/config"
)

type sidebarRow struct {
	Printer config.PrinterProfile
	Roll    config.LabelPreset
	Header  bool
	Empty   bool
	Spacer  bool
}

// Screen geometry of the sidebar list inside the left panel: the border takes one column, and the first
// row sits below the title and hint lines (verified against the rendered view in sidebar_test.go).
const (
	sidebarContentX = 1
	sidebarFirstRow = layoutBodyTop + 2
)

type chipAction int

const (
	chipNone chipAction = iota
	chipConnect
	chipDisconnect
	chipReconnect
)

// sidebarChip describes the right-aligned connection chip on a printer header and what clicking it does.
func sidebarChip(m Model, printer config.PrinterProfile) (string, lipgloss.Style, chipAction) {
	if printer.Name == m.Print.Printer {
		switch m.Connection {
		case ConnectionConnected:
			return "● connected", lipgloss.NewStyle().Foreground(lipgloss.Color("42")), chipDisconnect
		case ConnectionConnecting:
			return "◐ connecting", lipgloss.NewStyle().Foreground(lipgloss.Color("215")), chipNone
		case ConnectionDisconnected:
			return "○ reconnect", lipgloss.NewStyle().Foreground(lipgloss.Color("203")), chipReconnect
		default:
			return "○ unavailable", mutedStyle, chipNone
		}
	}
	switch {
	case m.DetectedNames == nil:
		return "○ connect", mutedStyle, chipConnect
	case m.DetectedNames[printer.Name]:
		return "○ connect", lipgloss.NewStyle().Foreground(catColor(mochaSky)), chipConnect
	default:
		return "· not seen", mutedStyle, chipConnect
	}
}

// sidebarChipSpan returns the half-open screen column range of a header's chip within the sidebar content.
func sidebarChipSpan(m Model, printer config.PrinterProfile, width int) (int, int) {
	text, _, _ := sidebarChip(m, printer)
	w := lipgloss.Width(text)
	return width - w, width
}

func (m *Model) activateSidebarChip(printer config.PrinterProfile) tea.Cmd {
	_, _, action := sidebarChip(*m, printer)
	switch action {
	case chipDisconnect:
		m.disconnectPrinter()
	case chipReconnect:
		return m.reconnectPrinter()
	case chipConnect:
		return m.applyPrinter(printer)
	default:
		m.setStatus("Printer is %s.", m.Connection)
	}
	return nil
}

func (m Model) sidebarVisibleRange(rows []sidebarRow) (int, int) {
	return sidebarWindow(len(rows), m.SidebarIndex, max(1, m.canvasPanelHeight()-8))
}

// clickSidebar handles a left click at screen (x, y) on the focused sidebar list.
func (m *Model) clickSidebar(x, y int) tea.Cmd {
	rows := m.sidebarRows()
	start, _ := m.sidebarVisibleRange(rows)
	index := start + y - sidebarFirstRow
	if y < sidebarFirstRow || index < 0 || index >= len(rows) {
		return nil
	}
	row := rows[index]
	if row.Empty || row.Spacer {
		return nil
	}
	m.SidebarIndex = index
	if !row.Header {
		return m.selectSidebarRow(row)
	}
	width := panelContentWidth(layoutLeftPanelWidth)
	chipStart, chipEnd := sidebarChipSpan(*m, row.Printer, width)
	col := x - sidebarContentX
	if col >= chipStart && col < chipEnd {
		return m.activateSidebarChip(row.Printer)
	}
	m.foldSidebarPrinter(row.Printer.Name, !m.SidebarCollapsed[row.Printer.Name])
	return nil
}

func (m Model) sidebarRows() []sidebarRow {
	rows := make([]sidebarRow, 0, len(m.Print.Printers)+len(m.AllPresets))
	for i, printer := range m.Print.Printers {
		if i > 0 {
			rows = append(rows, sidebarRow{Spacer: true})
		}
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
			if !rows[m.SidebarIndex].Empty && !rows[m.SidebarIndex].Spacer {
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
		if row, ok := m.currentSidebarRow(); ok && !row.Empty && !row.Spacer {
			return m.selectSidebarRow(row)
		}
	case "c":
		if row, ok := m.currentSidebarRow(); ok && !row.Empty && !row.Spacer {
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
	start, end := m.sidebarVisibleRange(rows)
	for i := start; i < end; i++ {
		row := rows[i]
		if row.Spacer {
			lines = append(lines, "")
			continue
		}
		prefix := "  "
		if i == m.SidebarIndex {
			prefix = "> "
		}
		if row.Header {
			arrow := "▾ "
			if m.SidebarCollapsed[row.Printer.Name] {
				arrow = "▸ "
			}
			chip, chipStyle, _ := sidebarChip(m, row.Printer)
			name := truncateText(row.Printer.Name, max(width-lipgloss.Width(chip)-lipgloss.Width(prefix+arrow)-1, 1))
			pad := strings.Repeat(" ", max(width-lipgloss.Width(prefix+arrow+name)-lipgloss.Width(chip), 1))
			nameStyle := lipgloss.NewStyle().Foreground(catColor(mochaMauve)).Bold(row.Printer.Name == m.Print.Printer)
			lines = append(lines, prefix+arrow+nameStyle.Render(name)+pad+chipStyle.Render(chip))
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
	lines = append(lines, helpItem("r", "rescan"), mutedStyle.Render("←/→ fold · click ○ to connect"))
	return padLines(lines, width)
}

func sidebarFooter(m Model) string {
	if !m.SidebarFocused {
		return ""
	}
	return strings.Join([]string{helpItem("↑/↓ k/j", "browse"), helpItem("←/→ h/l", "fold"), helpItem("enter", "select"), helpItem("tab", "editor")}, "  ")
}
