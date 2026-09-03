package tui

import (
	"fmt"
	"strings"
)

func (m Model) View() string {
	if !m.Ready {
		return "Loading label designer..."
	}

	toolWidth := 18
	gap := 2
	propertiesWidth := 28
	canvasLines := strings.Split(renderCanvas(m), "\n")
	toolLines := toolPanelLines(toolWidth)
	propertyLines := propertyPanelLines(m, propertiesWidth)
	bodyHeight := max(max(len(toolLines), len(canvasLines)), len(propertyLines))

	for len(toolLines) < bodyHeight {
		toolLines = append(toolLines, strings.Repeat(" ", toolWidth))
	}
	for len(canvasLines) < bodyHeight {
		canvasLines = append(canvasLines, strings.Repeat(" ", m.Canvas.Width))
	}
	for len(propertyLines) < bodyHeight {
		propertyLines = append(propertyLines, strings.Repeat(" ", propertiesWidth))
	}

	lines := []string{
		"Niimcli Label Designer",
		"",
	}
	for i := 0; i < bodyHeight; i++ {
		lines = append(lines, toolLines[i]+strings.Repeat(" ", gap)+canvasLines[i]+strings.Repeat(" ", gap)+propertyLines[i])
	}
	lines = append(lines, strings.Repeat("─", max(m.Width, 24)))
	lines = append(lines, fitLine(m.Status+" • t add • enter edit • +/- font • q quit • esc clear", max(m.Width, 24)))

	return strings.Join(lines, "\n")
}

func renderCanvas(m Model) string {
	canvas := m.Canvas
	if canvas.Width < 2 || canvas.Height < 2 {
		return ""
	}

	grid := make([][]rune, canvas.Height)
	for y := range grid {
		grid[y] = make([]rune, canvas.Width)
		for x := range grid[y] {
			grid[y][x] = ' '
		}
	}

	grid[0][0] = '┌'
	grid[0][canvas.Width-1] = '┐'
	grid[canvas.Height-1][0] = '└'
	grid[canvas.Height-1][canvas.Width-1] = '┘'
	for x := 1; x < canvas.Width-1; x++ {
		grid[0][x] = '─'
		grid[canvas.Height-1][x] = '─'
	}
	for y := 1; y < canvas.Height-1; y++ {
		grid[y][0] = '│'
		grid[y][canvas.Width-1] = '│'
	}

	for _, element := range m.Document.Elements {
		r := m.elementScreenRect(element)
		left := r.left - canvas.X
		top := r.top - canvas.Y
		right := r.right - canvas.X
		bottom := r.bottom - canvas.Y
		if bottom <= top || right <= left {
			continue
		}

		corner := '┌'
		if m.SelectedID == element.ID {
			corner = '●'
		}

		grid[top][left] = corner
		grid[top][right] = corner
		grid[bottom][left] = corner
		grid[bottom][right] = corner
		for x := left + 1; x < right; x++ {
			grid[top][x] = '─'
			grid[bottom][x] = '─'
		}
		for y := top + 1; y < bottom; y++ {
			grid[y][left] = '│'
			grid[y][right] = '│'
		}

		if element.Text != nil {
			textY := top + (bottom-top)/2
			text := truncateText(element.Text.Value, max(right-left-1, 0))
			textX := left + max(1, (right-left-len([]rune(text)))/2)
			for i, r := range []rune(text) {
				x := textX + i
				if x >= right {
					break
				}
				grid[textY][x] = r
			}
		}
	}

	lines := make([]string, 0, canvas.Height)
	for y := 0; y < canvas.Height; y++ {
		lines = append(lines, string(grid[y]))
	}
	return strings.Join(lines, "\n")
}

func toolPanelLines(width int) []string {
	return padLines([]string{
		"Tools",
		"",
		"[T] Text",
		"[Enter] Edit",
		"[Del] Delete",
		"[Arrows] Move",
		"[+/-] Font",
	}, width)
}

func propertyPanelLines(m Model, width int) []string {
	lines := []string{
		"Properties",
		"",
		fmt.Sprintf("Label W: %.1f mm", m.Document.WidthMM),
		fmt.Sprintf("Label H: %.1f mm", m.Document.HeightMM),
		"",
	}

	element, ok := m.selectedElement()
	if !ok {
		lines = append(lines, "No selection")
		if m.EditingText {
			lines = append(lines, "", "Editing:", m.TextBuffer)
		}
		return padLines(lines, width)
	}

	lines = append(lines,
		"Type: Text",
		fmt.Sprintf("X: %.1f mm", element.XMM),
		fmt.Sprintf("Y: %.1f mm", element.YMM),
		fmt.Sprintf("W: %.1f mm", element.WidthMM),
		fmt.Sprintf("H: %.1f mm", element.HeightMM),
	)
	if element.Text != nil {
		lines = append(lines,
			"",
			"Text:",
			element.Text.Value,
			fmt.Sprintf("Font: %.0f", element.Text.FontSize),
		)
	}
	if m.EditingText {
		lines = append(lines,
			"",
			"Editing:",
			m.TextBuffer,
		)
	}

	return padLines(lines, width)
}

func padLines(lines []string, width int) []string {
	padded := make([]string, 0, len(lines))
	for _, line := range lines {
		padded = append(padded, fitLine(line, width))
	}
	return padded
}

func fitLine(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func truncateText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
