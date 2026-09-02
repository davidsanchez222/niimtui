package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle    = lipgloss.NewStyle().Padding(0, 1)
	titleStyle  = lipgloss.NewStyle().Bold(true)
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	statusStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).Padding(0, 1)
	mutedStyle  = lipgloss.NewStyle().Faint(true)
)

func (m Model) View() string {
	if !m.Ready {
		return "Loading label designer..."
	}

	tools := panelStyle.Width(14).Render(strings.Join([]string{
		titleStyle.Render("Tools"),
		"",
		"[T] Text",
	}, "\n"))

	canvasTitle := titleStyle.Render(fmt.Sprintf("Label %.1fmm x %.1fmm", m.Document.WidthMM, m.Document.HeightMM))
	canvas := panelStyle.Render(strings.Join([]string{
		canvasTitle,
		"",
		renderCanvas(m.Canvas),
	}, "\n"))

	properties := panelStyle.Width(22).Render(strings.Join([]string{
		titleStyle.Render("Properties"),
		"",
		fmt.Sprintf("Width:  %.1f mm", m.Document.WidthMM),
		fmt.Sprintf("Height: %.1f mm", m.Document.HeightMM),
		"",
		mutedStyle.Render("No selection"),
	}, "\n"))

	body := lipgloss.JoinHorizontal(lipgloss.Top, tools, canvas, properties)
	status := statusStyle.Width(max(m.Width-2, 10)).Render(m.Status + " • q quit")

	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left, titleStyle.Render("Niimcli Label Designer"), "", body, status))
}

func renderCanvas(canvas Canvas) string {
	if canvas.Width < 2 || canvas.Height < 2 {
		return ""
	}

	var b strings.Builder
	b.Grow(canvas.Width * canvas.Height)

	innerWidth := canvas.Width - 2
	innerHeight := canvas.Height - 2

	b.WriteString("┌" + strings.Repeat("─", innerWidth) + "┐\n")
	for y := 0; y < innerHeight; y++ {
		b.WriteString("│" + strings.Repeat(" ", innerWidth) + "│")
		if y < innerHeight-1 {
			b.WriteByte('\n')
		}
	}
	if innerHeight > 0 {
		b.WriteByte('\n')
	}
	b.WriteString("└" + strings.Repeat("─", innerWidth) + "┘")

	return b.String()
}
