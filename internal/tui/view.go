package tui

import (
	"fmt"
	"image"
	"strings"

	"niimcli/internal/label"
	"niimcli/internal/render"
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
	lines = append(lines, fitLine(m.Status+" • t add • enter edit • +/- font • p preview • q quit • esc clear", max(m.Width, 24)))

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

	drawDocumentPreview(grid, canvas, m.Document)

	for _, element := range m.Document.Elements {
		r := m.elementScreenRect(element)
		left := r.left - canvas.X
		top := r.top - canvas.Y
		right := r.right - canvas.X
		bottom := r.bottom - canvas.Y
		if bottom <= top || right <= left {
			continue
		}

		grid[top][left] = '┌'
		grid[top][right] = '┐'
		grid[bottom][left] = '└'
		grid[bottom][right] = '┘'
		for x := left + 1; x < right; x++ {
			grid[top][x] = '─'
			grid[bottom][x] = '─'
		}
		for y := top + 1; y < bottom; y++ {
			grid[y][left] = '│'
			grid[y][right] = '│'
		}

		if m.SelectedID == element.ID && m.EditingText {
			drawEditingCursor(grid, left, top, right, bottom, element, m.TextBuffer)
		}

		if m.SelectedID == element.ID {
			for _, handle := range m.handlePoints(element) {
				hx := handle.x - canvas.X
				hy := handle.y - canvas.Y
				if hy >= 0 && hy < len(grid) && hx >= 0 && hx < len(grid[hy]) {
					grid[hy][hx] = '●'
				}
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
		"[t] Text",
		"[i] Edit",
		"[del] Delete",
		"[hjkl] Move",
		"[+/-] Font",
		"[p] Preview",
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

func drawDocumentPreview(grid [][]rune, canvas Canvas, doc label.Document) {
	result, err := render.RenderDocumentForTUI(doc)
	if err != nil {
		return
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		return
	}

	innerWidth := canvas.Width - 2
	innerHeight := canvas.Height - 2
	if innerWidth <= 0 || innerHeight <= 0 {
		return
	}

	const (
		brailleCols = 2
		brailleRows = 4
	)

	for cellY := 0; cellY < innerHeight; cellY++ {
		for cellX := 0; cellX < innerWidth; cellX++ {
			r := brailleRune(gray, cellX, cellY, innerWidth, innerHeight, brailleCols, brailleRows)
			if r == 0 {
				continue
			}
			grid[cellY+1][cellX+1] = r
		}
	}
}

func brailleRune(gray *image.Gray, cellX, cellY, innerWidth, innerHeight, subCols, subRows int) rune {
	base := rune(0x2800)
	bits := 0
	for sy := 0; sy < subRows; sy++ {
		for sx := 0; sx < subCols; sx++ {
			pixelMinX := gray.Bounds().Min.X + ((cellX*subCols+sx)*gray.Bounds().Dx())/(innerWidth*subCols)
			pixelMaxX := gray.Bounds().Min.X + ((cellX*subCols+sx+1)*gray.Bounds().Dx())/(innerWidth*subCols)
			pixelMinY := gray.Bounds().Min.Y + ((cellY*subRows+sy)*gray.Bounds().Dy())/(innerHeight*subRows)
			pixelMaxY := gray.Bounds().Min.Y + ((cellY*subRows+sy+1)*gray.Bounds().Dy())/(innerHeight*subRows)
			if pixelMaxX <= pixelMinX {
				pixelMaxX = pixelMinX + 1
			}
			if pixelMaxY <= pixelMinY {
				pixelMaxY = pixelMinY + 1
			}

			on := false
			for py := pixelMinY; py < pixelMaxY && !on; py++ {
				for px := pixelMinX; px < pixelMaxX; px++ {
					if gray.GrayAt(px, py).Y < 128 {
						on = true
						break
					}
				}
			}
			if on {
				bits |= brailleBit(sx, sy)
			}
		}
	}
	if bits == 0 {
		return 0
	}
	return base + rune(bits)
}

func brailleBit(x, y int) int {
	switch {
	case x == 0 && y == 0:
		return 0x01
	case x == 0 && y == 1:
		return 0x02
	case x == 0 && y == 2:
		return 0x04
	case x == 1 && y == 0:
		return 0x08
	case x == 1 && y == 1:
		return 0x10
	case x == 1 && y == 2:
		return 0x20
	case x == 0 && y == 3:
		return 0x40
	case x == 1 && y == 3:
		return 0x80
	default:
		return 0
	}
}

func drawEditingCursor(grid [][]rune, left, top, right, bottom int, element label.Element, text string) {
	if bottom <= top || right <= left {
		return
	}
	contentWidth := max(right-left-1, 1)
	previewLines := wrapCanvasText(element, text+"|", contentWidth)
	if len(previewLines) == 0 {
		previewLines = []string{"|"}
	}
	y := top + 1 + min(len(previewLines)-1, max(bottom-top-1, 0))
	if y >= bottom {
		y = bottom - 1
	}
	x := left + 1
	for i, r := range []rune(previewLines[len(previewLines)-1]) {
		cellX := x + i
		if cellX >= right {
			break
		}
		grid[y][cellX] = r
	}
}

func wrapCanvasText(element label.Element, text string, contentWidth int) []string {
	if contentWidth <= 0 || element.Text == nil {
		return nil
	}
	layout, err := render.LayoutTextWithFontPath(text, element.Text.FontSize, element.Text.FontPath, max(int(element.WidthMM*8), 1))
	if err != nil || len(layout.Lines) == 0 {
		if text == "" {
			return nil
		}
		return []string{truncateText(text, contentWidth)}
	}
	lines := make([]string, 0, len(layout.Lines))
	for _, line := range layout.Lines {
		lines = append(lines, truncateText(line, contentWidth))
	}
	return lines
}
