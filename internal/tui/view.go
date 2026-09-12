package tui

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"niimtui/internal/label"
	"niimtui/internal/render"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			Padding(0, 1)

	canvasStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	propertyTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("111"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	headerWordStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	headerSubStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("111"))

	footerRuleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))
)

func (m Model) View() string {
	if !m.Ready {
		return "Loading label designer..."
	}

	leftWidth := 30
	gap := 2
	propertiesWidth := 28
	canvasLines := strings.Split(renderCanvas(m), "\n")
	leftLines := devicePanelLines(m, leftWidth)
	propertyLines := propertyPanelLines(m, propertiesWidth)
	bodyHeight := max(max(len(leftLines), len(canvasLines)), len(propertyLines))

	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, strings.Repeat(" ", leftWidth))
	}
	for len(canvasLines) < bodyHeight {
		canvasLines = append(canvasLines, strings.Repeat(" ", m.Canvas.Width))
	}
	for len(propertyLines) < bodyHeight {
		propertyLines = append(propertyLines, strings.Repeat(" ", propertiesWidth))
	}

	viewWidth := max(m.Width, leftWidth+gap+m.Canvas.Width+gap+propertiesWidth)
	title := lipgloss.PlaceHorizontal(viewWidth, lipgloss.Center, logoHeader())
	lines := []string{
		title,
		"",
	}
	for i := 0; i < bodyHeight; i++ {
		lines = append(lines, leftLines[i]+strings.Repeat(" ", gap)+canvasStyle.Render(canvasLines[i])+strings.Repeat(" ", gap)+propertyLines[i])
	}
	lines = append(lines, footerLines(m, viewWidth)...)

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
	if strings.EqualFold(m.Document.Shape, "round") {
		drawRoundGuide(grid, canvas)
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

func devicePanelLines(m Model, width int) []string {
	lines := []string{
		propertyTitleStyle.Render("Printer"),
		"",
		"┌──────────────────────┐",
		"│                      │",
		"│       printer        │",
		"│     placeholder      │",
		"│                      │",
		"└──────────────────────┘",
		"",
		propertyItem("Model", sidebarValue("Model", m.Print.Model, "none", width)),
		propertyItem("Profile", sidebarValue("Profile", m.Print.Printer, "default", width)),
		propertyItem("Device", sidebarValue("Device", m.Print.DeviceName, emptyFallback(m.Print.Identifier, "unknown"), width)),
		"",
		connectionStatusLine(m),
	}
	if matched := connectionMetaString(m.ConnectMeta, "matched_name"); matched != "" && matched != m.Print.DeviceName {
		lines = append(lines, propertyItem("BLE", truncateText(matched, sidebarValueWidth("BLE", width))))
	}
	if address := connectionMetaString(m.ConnectMeta, "address"); address != "" {
		lines = append(lines, propertyItem("Addr", truncateText(address, sidebarValueWidth("Addr", width))))
	}
	if m.ConnectErr != "" {
		lines = append(lines, "", propertyLabel("Error"), truncateText(m.ConnectErr, width))
	}
	return padLines(lines, width)
}

func propertyPanelLines(m Model, width int) []string {
	lines := []string{
		propertyTitleStyle.Render("Properties"),
		"",
		propertyItem("Label W", fmt.Sprintf("%.1f mm", m.Document.WidthMM)),
		propertyItem("Label H", fmt.Sprintf("%.1f mm", m.Document.HeightMM)),
		propertyItem("Shape", m.Document.Shape),
		propertyItem("Print", printDirectionLabel(m.Print.Model)),
		"",
	}

	element, ok := m.selectedElement()
	if !ok {
		lines = append(lines, mutedStyle.Render("No selection"))
		if m.EditingText {
			lines = append(lines, "", propertyLabel("Editing"), m.TextBuffer)
		}
		return padLines(lines, width)
	}

	lines = append(lines,
		propertyItem("Type", elementTypeLabel(element)),
		propertyItem("X", fmt.Sprintf("%.1f mm", element.XMM)),
		propertyItem("Y", fmt.Sprintf("%.1f mm", element.YMM)),
		propertyItem("W", fmt.Sprintf("%.1f mm", element.WidthMM)),
		propertyItem("H", fmt.Sprintf("%.1f mm", element.HeightMM)),
	)
	if element.Text != nil {
		lines = append(lines,
			"",
			propertyItem("Text", element.Text.Value),
			propertyItem("Font", fmt.Sprintf("%.0f", element.Text.FontSize)),
		)
	}
	if element.QR != nil {
		lines = append(lines,
			"",
			propertyLabel("QR"),
			element.QR.Value,
		)
	}
	if m.EditingText {
		lines = append(lines,
			"",
			propertyLabel("Editing"),
			m.TextBuffer,
		)
	}

	return padLines(lines, width)
}

func footerLines(m Model, width int) []string {
	printHelp := ""
	if m.Print.Session != nil {
		printHelp = helpItem("P", "print") + "  "
	}
	controls := strings.Join([]string{
		helpItem("t", "text"),
		helpItem("r", "QR"),
		helpItem("i", "edit"),
		helpItem("m", "mouse"),
		helpItem("del", "remove"),
		helpItem("hjkl", "move"),
		helpItem("shift+arrows", "fast move"),
		helpItem("+/-", "font"),
	}, "  ")
	preview := strings.Join([]string{
		helpItem("p", "preview"),
		printHelp + reconnectHelp(m) + helpItem("esc", "clear"),
		helpItem("q", "quit"),
		helpItem("ctrl+c", "quit"),
	}, "  ")

	return []string{
		footerRuleStyle.Render(strings.Repeat("─", max(width, 24))),
		centerStyledLine(statusStyle.Render(m.Status), width),
		centerStyledLine(controls, width),
		centerStyledLine(preview, width),
	}
}

func propertyItem(label, value string) string {
	return propertyLabel(label) + " " + value
}

func propertyLabel(label string) string {
	return keyStyle.Render(label + ":")
}

func reconnectHelp(m Model) string {
	if m.Print.Session == nil {
		return ""
	}
	return helpItem("c", "reconnect") + "  "
}

func emptyFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func sidebarValue(label, value, fallback string, width int) string {
	return truncateText(emptyFallback(value, fallback), sidebarValueWidth(label, width))
}

func sidebarValueWidth(label string, width int) int {
	return max(width-len(label)-3, 1)
}

func connectionStatusLine(m Model) string {
	switch m.Connection {
	case ConnectionConnected:
		return connectionDotStyle("42").Render("●") + helpLabelStyle.Render(" connected")
	case ConnectionConnecting:
		return connectionDotStyle("229").Render("●") + helpLabelStyle.Render(" connecting")
	case ConnectionDisconnected:
		return connectionDotStyle("203").Render("●") + helpLabelStyle.Render(" disconnected")
	default:
		return mutedStyle.Render("● unavailable")
	}
}

func connectionDotStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

func connectionMetaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, ok := meta[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func logoHeader() string {
	return titleStyle.Render("›_") + " " + headerWordStyle.Render("niimtui") + headerSubStyle.Render("  terminal label designer")
}

func elementTypeLabel(element label.Element) string {
	if element.QR != nil {
		return "QR"
	}
	if element.Text != nil {
		return "Text"
	}
	return string(element.Type)
}

func printDirectionLabel(model string) string {
	switch strings.ToUpper(strings.TrimSpace(model)) {
	case "B1", "B18", "B21":
		return "bottom to top"
	case "D110", "D110_M", "D11":
		return "left to right"
	case "":
		return "unknown"
	default:
		return "model default"
	}
}

func helpItem(key, label string) string {
	return keyStyle.Render(key) + helpLabelStyle.Render(" "+label)
}

func padLines(lines []string, width int) []string {
	padded := make([]string, 0, len(lines))
	for _, line := range lines {
		padded = append(padded, fitStyledLine(line, width))
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

func fitStyledLine(s string, width int) string {
	lineWidth := lipgloss.Width(s)
	if lineWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-lineWidth)
}

func centerStyledLine(s string, width int) string {
	lineWidth := lipgloss.Width(s)
	if lineWidth >= width {
		return s
	}
	leftPadding := (width - lineWidth) / 2
	rightPadding := width - lineWidth - leftPadding
	return strings.Repeat(" ", leftPadding) + s + strings.Repeat(" ", rightPadding)
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

func drawRoundGuide(grid [][]rune, canvas Canvas) {
	innerWidth := canvas.Width - 2
	innerHeight := canvas.Height - 2
	if innerWidth <= 2 || innerHeight <= 2 {
		return
	}

	visualWidth := float64(innerWidth) * terminalCellWidthToHeightRatio
	visualHeight := float64(innerHeight)
	radius := math.Min(visualWidth, visualHeight) / 2
	if radius <= 0 {
		return
	}
	cx := visualWidth / 2
	cy := visualHeight / 2
	tolerance := math.Max(0.35, radius*0.08)

	for y := 0; y < innerHeight; y++ {
		for x := 0; x < innerWidth; x++ {
			vx := (float64(x) + 0.5) * terminalCellWidthToHeightRatio
			vy := float64(y) + 0.5
			distance := math.Hypot(vx-cx, vy-cy)
			if math.Abs(distance-radius) <= tolerance {
				grid[y+1][x+1] = '·'
			}
		}
	}
}

func drawEditingCursor(grid [][]rune, left, top, right, bottom int, element label.Element, text string) {
	if bottom <= top || right <= left {
		return
	}
	contentWidth := max(right-left-1, 1)
	if element.QR != nil {
		grid[top+1][left+1] = '>'
		for i, r := range []rune(truncateText(text+"|", max(contentWidth-1, 1))) {
			cellX := left + 2 + i
			if cellX >= right {
				break
			}
			grid[top+1][cellX] = r
		}
		return
	}
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
