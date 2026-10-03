package tui

import (
	"fmt"
	"image"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaPeach)).
			Padding(0, 1)

	canvasStyle = lipgloss.NewStyle().
			Foreground(catColor(mochaText))

	printableGuideStyle = lipgloss.NewStyle().
				Foreground(catColor(mochaRed))

	printDirectionStyle = lipgloss.NewStyle().
				Foreground(catColor(mochaSky)).
				Bold(true)

	gridStyle = lipgloss.NewStyle().
			Foreground(catColor(mochaRed))

	propertyTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(catColor(mochaMauve))

	propertySelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(catColor(mochaPeach)).
				Background(catColor(mochaSurface0))

	propertyLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(catColor(mochaYellow))

	mutedStyle = lipgloss.NewStyle().
			Foreground(catColor(mochaOverlay1))

	helpLabelStyle = lipgloss.NewStyle().
			Foreground(catColor(mochaSubtext1))

	statusStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaCrust)).
			Background(catColor(mochaPeach)).
			Padding(0, 1)

	focusHintStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaCrust)).
			Background(catColor(mochaPink))

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaMauve))

	headerWordStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaText))

	headerSubStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaLavender))

	footerRuleStyle = lipgloss.NewStyle().
			Foreground(catColor(mochaSurface1))

	editInputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(catColor(mochaMauve)).
			Padding(0, 1)

	panelBorderStyle = lipgloss.NewStyle().
				Foreground(catColor(mochaSurface2))

	panelActiveBorderStyle = lipgloss.NewStyle().
				Foreground(catColor(mochaMauve))

	topBarBorderStyle = lipgloss.NewStyle().
				Foreground(catColor(mochaSurface1))

	topBarRuleStyle = lipgloss.NewStyle().
			Foreground(catColor(mochaMauve))

	topBarActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(catColor(mochaMauve))

	topBarInactiveStyle = lipgloss.NewStyle().
				Foreground(catColor(mochaOverlay2))

	topBarHoverStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(catColor(mochaMauve)).
				Background(catColor(mochaSurface0))

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(catColor(mochaText))
)

const (
	minTerminalWidth      = 140
	minTerminalHeight     = 30
	layoutLeftPanelWidth  = 32
	layoutPropertiesWidth = 30
	layoutPanelGap        = 1
	layoutTopBarHeight    = 3
	layoutStatusHeight    = 1
	layoutFooterHeight    = 4
	layoutBodyTop         = layoutTopBarHeight + layoutStatusHeight + 1
	printableGuideRune    = '┊'
	printableGuideHorz    = '┈'
	gridHorizontalRune    = '┄'
	gridVerticalRune      = '┊'
	gridIntersectionRune  = '┼'
	promptCursorRune      = '█'
)

func (m Model) View() string {
	return highlightSelection(m.view(), m.Selection)
}

func (m Model) view() string {
	if !m.Ready {
		return "Loading label designer..."
	}
	if m.isTerminalTooSmall() {
		return smallTerminalView(m.Width, m.Height)
	}

	canvasPanelWidth := m.canvasPanelWidth()
	bodyHeight := m.canvasPanelHeight()
	var canvasLines []string
	if m.Tab == tabGallery {
		canvasLines = galleryCanvasLines(m, canvasPanelWidth, bodyHeight)
	} else {
		canvasLines = fitPanelLines(centerCanvasLines(strings.Split(renderCanvas(m), "\n"), canvasPanelWidth, bodyHeight), bodyHeight, canvasPanelWidth)
		if m.EditingText {
			canvasLines = overlayCenteredBox(canvasLines, editingPopup(m, canvasPanelWidth, bodyHeight), canvasPanelWidth, bodyHeight)
		}
	}
	leftContentWidth := panelContentWidth(layoutLeftPanelWidth)
	rightContentWidth := panelContentWidth(layoutPropertiesWidth)
	leftPanel := devicePanelLines(m, leftContentWidth)
	if m.Tab == tabGallery {
		leftPanel = galleryListLines(m, leftContentWidth)
	} else if m.SidebarFocused {
		leftPanel = sidebarLines(m, leftContentWidth)
	}
	leftLines := fitPanelLines(leftPanel, bodyHeight, leftContentWidth)
	var properties []string
	if m.Tab == tabGallery {
		properties = m.galleryPreviewLines(rightContentWidth)
	} else {
		properties = propertyPanelLines(m, rightContentWidth)
	}
	propertyLines := fitPanelLines(properties, bodyHeight, rightContentWidth)

	viewWidth := max(m.Width, layoutLeftPanelWidth+layoutPanelGap+canvasPanelWidth+2+layoutPanelGap+layoutPropertiesWidth)
	lines := topBarLines(m, viewWidth)
	lines = append(lines, statusLine(m, viewWidth))
	if m.HelpOpen || m.MenuOpen || m.confirmPromptOpen() {
		lines = append(lines, modalBodyLines(m, viewWidth, bodyHeight)...)
		lines = append(lines, footerLines(m, viewWidth)...)
		return strings.Join(lines, "\n")
	}
	leftTitle, centerTitle, rightTitle := panelTitles(m)
	leftBox := panelLines(leftTitle, leftLines, layoutLeftPanelWidth, bodyHeight+2, m.SidebarFocused || m.Tab == tabGallery)
	centerBox := panelLines(centerTitle, canvasLines, canvasPanelWidth+2, bodyHeight+2, !m.SidebarFocused && m.Tab == tabDesigner)
	rightBox := panelLines(rightTitle, propertyLines, layoutPropertiesWidth, bodyHeight+2, false)
	for i := range leftBox {
		lines = append(lines, leftBox[i]+strings.Repeat(" ", layoutPanelGap)+centerBox[i]+strings.Repeat(" ", layoutPanelGap)+rightBox[i])
	}
	lines = append(lines, footerLines(m, viewWidth)...)

	return strings.Join(lines, "\n")
}

func topBarLines(m Model, width int) []string {
	contentWidth := max(width-2, 1)
	content := topBarContent(m, contentWidth)
	return []string{
		topBarBorderStyle.Render("╭" + strings.Repeat("─", contentWidth) + "╮"),
		topBarBorderStyle.Render("│") + content + topBarBorderStyle.Render("│"),
		topBarBorderStyle.Render("╰" + strings.Repeat("─", contentWidth) + "╯"),
	}
}

func topBarContent(m Model, width int) string {
	brand := logoHeader()
	controls := tabHeader(m, width)
	line := fitStyledLine(brand, width)
	left := max((width-lipgloss.Width(topBarControlsText()))/2, 0)
	line = overlayStyledLine(line, controls, left, width)
	quitLeft := max(width-lipgloss.Width(topBarQuitText())-1, 0)
	return overlayStyledLine(line, topBarQuitControl(m.TopBarHover == topBarQuit), quitLeft, width)
}

func topBarControlsText() string {
	return "1 Designer  |  2 Gallery  |  3 Menu"
}

func topBarQuitText() string {
	return "ctrl+c quit"
}

func topBarQuitControl(hover bool) string {
	key := topBarActiveStyle.Render("ctrl+c")
	labelStyle := topBarInactiveStyle
	if hover {
		labelStyle = topBarHoverStyle
	}
	return key + labelStyle.Render(" quit")
}

func tabHeader(m Model, _ int) string {
	return strings.Join([]string{
		topBarControl("1 Designer", m.Tab == tabDesigner && !m.MenuOpen, m.TopBarHover == topBarDesigner),
		topBarControl("2 Gallery", m.Tab == tabGallery && !m.MenuOpen, m.TopBarHover == topBarGallery),
		topBarControl("3 Menu", m.MenuOpen, m.TopBarHover == topBarMenu),
	}, topBarInactiveStyle.Render("  |  "))
}

func topBarControl(label string, active, hover bool) string {
	if hover {
		return topBarHoverStyle.Render(label)
	}
	if active {
		return topBarActiveStyle.Render(label)
	}
	return topBarInactiveStyle.Render(label)
}

type topBarTarget uint8

const (
	topBarNone topBarTarget = iota
	topBarDesigner
	topBarGallery
	topBarMenu
	topBarQuit
)

func topBarTargetAt(width, x, y int) topBarTarget {
	if y != 1 {
		return topBarNone
	}
	contentWidth := max(width-2, 1)
	contentX := x - 1
	if contentX < 0 || contentX >= contentWidth {
		return topBarNone
	}
	controlsWidth := lipgloss.Width(topBarControlsText())
	left := max((contentWidth-controlsWidth)/2, 0)
	switch {
	case contentX >= left && contentX < left+10:
		return topBarDesigner
	case contentX >= left+15 && contentX < left+25:
		return topBarGallery
	case contentX >= left+30 && contentX < left+36:
		return topBarMenu
	}
	quitLeft := max(contentWidth-lipgloss.Width(topBarQuitText())-1, 0)
	if contentX >= quitLeft && contentX < quitLeft+lipgloss.Width(topBarQuitText()) {
		return topBarQuit
	}
	return topBarNone
}

func tabAt(width, x int) (tuiTab, bool) {
	controlsWidth := lipgloss.Width(topBarControlsText())
	left := 1 + max((max(width-2, 1)-controlsWidth)/2, 0)
	if x >= left && x < left+10 {
		return tabDesigner, true
	}
	if x >= left+15 && x < left+25 {
		return tabGallery, true
	}
	legacyLeft := (width - 26) / 2
	if x >= legacyLeft && x < legacyLeft+12 {
		return tabDesigner, true
	}
	if x >= legacyLeft+15 && x < legacyLeft+26 {
		return tabGallery, true
	}
	return tabDesigner, false
}

func quitAt(width, x int) bool {
	return topBarTargetAt(width, x, 1) == topBarQuit
}

func panelTitles(m Model) (string, string, string) {
	if m.Tab == tabGallery {
		return "Library", "Label Preview", "Details"
	}
	left := "Printer"
	if m.SidebarFocused {
		left = "Printers & Rolls"
	}
	return left, "Designer", "Inspector"
}

func panelLines(title string, body []string, width, height int, active bool) []string {
	width = max(width, 4)
	height = max(height, 2)
	contentWidth := panelContentWidth(width)
	border := panelBorderStyle
	if active {
		border = panelActiveBorderStyle
	}
	titleText := panelTitleStyle.Render(" " + title + " ")
	titleWidth := lipgloss.Width(" " + title + " ")
	ruleWidth := max(width-titleWidth-3, 0)
	top := border.Render("╭─") + titleText + border.Render(strings.Repeat("─", ruleWidth)+"╮")
	bottom := border.Render("╰" + strings.Repeat("─", width-2) + "╯")
	lines := []string{top}
	for i := 0; i < height-2; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		line = truncateStyledLine(line, contentWidth)
		lines = append(lines, border.Render("│")+fitStyledLine(line, contentWidth)+border.Render("│"))
	}
	lines = append(lines, bottom)
	return lines
}

func panelContentWidth(width int) int {
	return max(width-2, 1)
}

func statusLine(m Model, width int) string {
	return fitStyledLine(" "+statusStyle.Render(truncateText(m.Status, max(width-4, 1))), width)
}

func (m Model) isTerminalTooSmall() bool {
	return m.Width < minTerminalWidth || m.Height < minTerminalHeight
}

func (m Model) canvasPanelWidth() int {
	return max(m.Width-layoutLeftPanelWidth-layoutPropertiesWidth-(2*layoutPanelGap)-2, 12)
}

func (m Model) canvasPanelHeight() int {
	return max(m.Height-layoutTopBarHeight-layoutStatusHeight-layoutFooterHeight-2, 6)
}

func (m Model) canvasPanelLeft() int {
	return layoutLeftPanelWidth + layoutPanelGap + 1
}

func centerCanvasLines(lines []string, width, height int) []string {
	centered := make([]string, 0, height)
	topPadding := max((height-len(lines))/2, 0)
	for range topPadding {
		centered = append(centered, strings.Repeat(" ", width))
	}
	for _, line := range lines {
		centered = append(centered, centerStyledLine(line, width))
	}
	for len(centered) < height {
		centered = append(centered, strings.Repeat(" ", width))
	}
	return centered
}

func fitPanelLines(lines []string, height, width int) []string {
	fitted := make([]string, 0, height)
	for i := 0; i < height && i < len(lines); i++ {
		fitted = append(fitted, fitStyledLine(lines[i], width))
	}
	for len(fitted) < height {
		fitted = append(fitted, strings.Repeat(" ", width))
	}
	return fitted
}

func smallTerminalView(width, height int) string {
	message := strings.Join([]string{
		propertyTitleStyle.Render("Terminal size too small:"),
		fmt.Sprintf("Width = %s Height = %s", propertyLabelStyle.Render(fmt.Sprint(width)), propertyLabelStyle.Render(fmt.Sprint(height))),
		"",
		propertyTitleStyle.Render("Needed for current config:"),
		fmt.Sprintf("Width = %s Height = %s", propertyLabelStyle.Render(fmt.Sprint(minTerminalWidth)), propertyLabelStyle.Render(fmt.Sprint(minTerminalHeight))),
	}, "\n")
	return lipgloss.Place(max(width, 1), max(height, 1), lipgloss.Center, lipgloss.Center, message)
}

func modalBodyLines(m Model, width, height int) []string {
	content := helpModalContent()
	if m.confirmPromptOpen() {
		switch m.Prompt.Mode {
		case PromptCommand:
			content = []string{propertyTitleStyle.Render("Print this saved design"), "", mutedStyle.Render("c copies full command · ↑/↓ or k/j scroll · esc closes"), ""}
			wrapped := wrapCommand(m.Prompt.Value, 76)
			start := min(m.CommandScroll, max(0, len(wrapped)-max(1, height-9)))
			content = append(content, wrapped[start:min(len(wrapped), start+max(1, height-9))]...)
		default:
			verb := "Overwrite"
			title := "Overwrite saved preset?"
			if m.Prompt.Mode == PromptDeleteDesign {
				verb = "Delete"
				title = "Delete saved preset?"
			}
			if m.Prompt.Mode == PromptOpenGallery {
				verb = "Discard edits and open"
				title = "Discard unsaved edits?"
			}
			content = []string{propertyTitleStyle.Render(title), "", fmt.Sprintf("%s %q?", verb, m.Prompt.Value), "", mutedStyle.Render("↑/↓ or k/j choose · enter selects · y/n shortcuts"), ""}
			for i, option := range []string{"[n] Cancel", "[y] " + verb} {
				prefix := "  "
				if i == m.Prompt.Choice {
					prefix = "> "
				}
				style := lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
				if i == 1 {
					style = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
				}
				if i == m.Prompt.Choice {
					style = style.Bold(true).Background(lipgloss.Color("236"))
				}
				content = append(content, style.Render(prefix+option))
			}
		}
	} else if m.MenuOpen {
		content = menuModalContent(m)
	}
	modalWidth := min(max(width-16, 72), 96)
	modalWidth = min(modalWidth, max(width-4, 40))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("111")).
		Padding(1, 3).
		Width(modalWidth).
		Render(strings.Join(content, "\n"))
	body := lipgloss.Place(width, max(height, 8), lipgloss.Center, lipgloss.Center, box)
	return strings.Split(body, "\n")
}

func wrapCommand(command string, width int) []string {
	runes := []rune(command)
	lines := make([]string, 0, (len(runes)+width-1)/width)
	for len(runes) > 0 {
		n := min(len(runes), width)
		lines = append(lines, string(runes[:n]))
		runes = runes[n:]
	}
	return lines
}

func editingPopup(m Model, width, height int) string {
	inputWidth := min(max(width-18, 20), 72)
	inputWidth = min(inputWidth, max(width-6, 8))
	inputHeight := min(max(height-12, 1), 8)

	inputLines := editingTextLines(m.TextBuffer, inputWidth, inputHeight)
	for i, line := range inputLines {
		inputLines[i] = fitLine(line, inputWidth)
	}

	return editInputStyle.Width(inputWidth).Render(strings.Join(inputLines, "\n"))
}

func editingTextLines(value string, width, maxLines int) []string {
	width = max(width, 1)
	maxLines = max(maxLines, 1)

	parts := strings.Split(value+string(promptCursorRune), "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(runes) > 0 {
			take := min(width, len(runes))
			lines = append(lines, string(runes[:take]))
			runes = runes[take:]
		}
	}
	if len(lines) == 0 {
		lines = append(lines, string(promptCursorRune))
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func overlayCenteredBox(background []string, overlay string, width, height int) []string {
	result := append([]string(nil), background...)
	overlayLines := strings.Split(overlay, "\n")
	if len(overlayLines) == 0 || height <= 0 || width <= 0 {
		return result
	}
	overlayWidth := 0
	for _, line := range overlayLines {
		overlayWidth = max(overlayWidth, lipgloss.Width(line))
	}
	left := max((width-overlayWidth)/2, 0)
	top := max((height-len(overlayLines))/2, 0)

	for i, overlayLine := range overlayLines {
		row := top + i
		if row < 0 || row >= len(result) {
			continue
		}
		result[row] = overlayStyledLine(result[row], overlayLine, left, width)
	}
	return result
}

func overlayStyledLine(background, overlay string, left, width int) string {
	backgroundCells := styledCells(background)
	for len(backgroundCells) < width {
		backgroundCells = append(backgroundCells, " ")
	}
	overlayCells := styledCells(overlay)
	for i, cell := range overlayCells {
		column := left + i
		if column < 0 || column >= width || column >= len(backgroundCells) {
			continue
		}
		backgroundCells[column] = cell
	}
	return strings.Join(backgroundCells, "")
}

func styledCells(s string) []string {
	cells := []string{}
	for i := 0; i < len(s); {
		prefix := ""
		for i < len(s) && isANSIStart(s, i) {
			seq, next := readANSISequence(s, i)
			prefix += seq
			i = next
		}
		if i >= len(s) {
			if len(cells) > 0 {
				cells[len(cells)-1] += prefix
			}
			break
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		i += size
		cell := prefix + string(r)
		for i < len(s) && isANSIStart(s, i) {
			seq, next := readANSISequence(s, i)
			cell += seq
			i = next
		}
		cells = append(cells, cell)
	}
	return cells
}

func isANSIStart(s string, index int) bool {
	return index < len(s) && s[index] == '\x1b'
}

func readANSISequence(s string, start int) (string, int) {
	if start >= len(s) || s[start] != '\x1b' {
		return "", start
	}
	if start+1 < len(s) && s[start+1] == '[' {
		for i := start + 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return s[start : i+1], i + 1
			}
		}
		return s[start:], len(s)
	}
	if start+1 < len(s) && s[start+1] == ']' {
		for i := start + 2; i < len(s); i++ {
			if s[i] == '\a' {
				return s[start : i+1], i + 1
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				return s[start : i+2], i + 2
			}
		}
		return s[start:], len(s)
	}
	for i := start + 1; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return s[start : i+1], i + 1
		}
	}
	return s[start:], len(s)
}

func renderCanvas(m Model) string {
	canvas := m.Canvas
	if canvas.Width < 2 || canvas.Height < 2 {
		return ""
	}
	m.Document = m.previewDocument()
	isRound := strings.EqualFold(m.Document.Shape, "round")

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

	if m.Grid {
		drawGridGuide(grid, canvas)
	}
	drawDocumentPreview(grid, canvas, m.Document)
	if !isRound {
		drawPrintableAreaGuide(grid, canvas, m)
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
	focusCells := map[int]map[int]bool(nil)
	if m.FocusPickerOpen {
		focusCells = drawFocusHints(grid, canvas, m)
	}
	drawPrintDirectionGuide(grid, canvas, m)

	lines := make([]string, 0, canvas.Height)
	for y := 0; y < canvas.Height; y++ {
		lines = append(lines, renderCanvasLine(grid[y], focusCells[y]))
	}
	return strings.Join(lines, "\n")
}

func drawFocusHints(grid [][]rune, canvas Canvas, m Model) map[int]map[int]bool {
	focusCells := make(map[int]map[int]bool)
	for _, hint := range m.focusHints() {
		r := m.elementScreenRect(hint.Element)
		x := clampInt(r.left-canvas.X, 1, max(canvas.Width-2, 1))
		y := clampInt(r.top-canvas.Y-1, 1, max(canvas.Height-2, 1))
		runes := []rune(hint.Hint)
		if len(runes) == 0 || y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
			continue
		}
		grid[y][x] = runes[0]
		if focusCells[y] == nil {
			focusCells[y] = make(map[int]bool)
		}
		focusCells[y][x] = true
	}
	return focusCells
}

func renderCanvasLine(row []rune, focusCells map[int]bool) string {
	var b strings.Builder
	for x, r := range row {
		if focusCells[x] {
			b.WriteString(focusHintStyle.Render(string(r)))
			continue
		}
		if isPrintableGuideRune(r) {
			b.WriteString(printableGuideStyle.Render(string(r)))
			continue
		}
		if isGridRune(r) {
			b.WriteString(gridStyle.Render(string(r)))
			continue
		}
		if isPrintDirectionRune(r) {
			b.WriteString(printDirectionStyle.Render(string(r)))
			continue
		}
		b.WriteString(canvasStyle.Render(string(r)))
	}
	return b.String()
}

func drawGridGuide(grid [][]rune, canvas Canvas) {
	if canvas.Width < 6 || canvas.Height < 6 {
		return
	}
	for y := 3; y < canvas.Height-1; y += 3 {
		for x := 1; x < canvas.Width-1; x++ {
			if grid[y][x] == gridVerticalRune {
				grid[y][x] = gridIntersectionRune
				continue
			}
			if grid[y][x] == ' ' {
				grid[y][x] = gridHorizontalRune
			}
		}
	}
	for x := 6; x < canvas.Width-1; x += 6 {
		for y := 1; y < canvas.Height-1; y++ {
			if grid[y][x] == gridHorizontalRune {
				grid[y][x] = gridIntersectionRune
				continue
			}
			if grid[y][x] == ' ' {
				grid[y][x] = gridVerticalRune
			}
		}
	}
}

func isGridRune(r rune) bool {
	return r == gridHorizontalRune || r == gridVerticalRune || r == gridIntersectionRune
}

func isPrintableGuideRune(r rune) bool {
	return r == printableGuideRune || r == printableGuideHorz || r == '┬' || r == '┴' || r == '├' || r == '┤'
}

func isPrintDirectionRune(r rune) bool {
	return r == '▲' || r == '▶' || r == '▼' || r == '◀'
}

func devicePanelLines(m Model, width int) []string {
	lines := []string{}
	art := printerArt(m.Print.Model)
	if len(art) > 0 {
		lines = append(lines, artLines(art, width)...)
	}

	lines = append(lines,
		propertyTitleStyle.Render("Printer"),
		propertyItem("Model", sidebarValue("Model", m.Print.Model, "none", width)),
		propertyItem("Device", sidebarValue("Device", m.Print.DeviceName, emptyFallback(m.Print.Identifier, "unknown"), width)),
		"",
		propertyLabel("Installed"),
		propertyItem("Label", sidebarValue("Label", m.currentPresetLabel(), "custom", width)),
		presetSwitchHelp(m),
		"",
		connectionStatusLine(m),
		connectHelp(m),
	)
	installed := installedPrinterLines(m, width)
	if len(installed) > 0 {
		insertAt := len(lines) - 5
		lines = append(lines[:insertAt], append(installed, lines[insertAt:]...)...)
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

func (m Model) currentPresetLabel() string {
	if m.Preset >= 0 && m.Preset < len(m.Presets) {
		return m.Presets[m.Preset].Name
	}
	return fmt.Sprintf("%.0fx%.0f %s", m.Document.WidthMM, m.Document.HeightMM, m.Document.Shape)
}

func presetSwitchHelp(m Model) string {
	if len(m.Presets) == 0 {
		return helpItem("tab", "manage printer")
	}
	return helpItem("tab", "manage printer/roll")
}

func installedPrinterLines(m Model, width int) []string {
	if len(m.Print.Printers) == 0 {
		return []string{mutedStyle.Render("No printers installed"), ""}
	}
	limit := min(len(m.Print.Printers), 5)
	lines := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		printer := m.Print.Printers[i]
		marker := " "
		style := helpLabelStyle
		if printer.Name == m.Print.Printer {
			marker = "*"
			style = propertySelectedStyle
		}
		prefix := style.Render(" " + marker + " ")
		name := truncateText(printerDisplayName(printer), max(width-lipgloss.Width(prefix), 1))
		lines = append(lines, prefix+style.Render(name))
	}
	lines = append(lines, "")
	return lines
}

func artLines(lines []string, width int) []string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, truncateText(line, width))
	}
	return rendered
}

func propertyPanelLines(m Model, width int) []string {
	fontPickerElement, showFontPicker := m.selectedElement()
	showFontPicker = showFontPicker && fontPickerElement.Text != nil && m.FontPickerOpen
	lines := []string{
		propertyTitleStyle.Render("Live Preview"),
	}
	if m.hasTerminalLivePreview() {
		_, previewHeight := m.livePreviewPanelCellSize(width)
		for range previewHeight {
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, mutedStyle.Render("Press p to open preview"))
	}
	if showFontPicker {
		available := max(m.canvasPanelHeight()-len(lines)-2, 3)
		itemLimit := max(1, available-3)
		lines = append(lines,
			"",
			propertyTitleStyle.Render("Fonts"),
		)
		lines = append(lines, fontPickerLinesWithLimit(m, width, min(fontPickerPageSize, itemLimit))...)
	}
	lines = append(lines,
		propertyTitleStyle.Render("Properties"),
		propertyItem("Label W", fmt.Sprintf("%.1f mm", m.Document.WidthMM)),
		propertyItem("Label H", fmt.Sprintf("%.1f mm", m.Document.HeightMM)),
		propertyItem("Shape", m.Document.Shape),
		propertyItem("Canvas Rot", fmt.Sprintf("%d deg", m.Document.Rotation)),
		propertyItem("Invert", onOff(m.Document.Inverted)),
		propertyItem("Grid", onOff(m.Grid)),
		propertyItem("Print", printDirectionLabel(m.Print.Model)),
		"",
	)

	element, ok := m.selectedElement()
	if !ok {
		lines = append(lines, mutedStyle.Render("No selection"))
		if m.EditingText {
			lines = append(lines, "", propertyLabel("Editing"), m.TextBuffer)
		}
		return padLines(lines, width)
	}
	if showFontPicker {
		if preview, ok := m.previewDocument().ElementByID(element.ID); ok {
			element = preview
		}
	}

	lines = append(lines,
		propertyItem("Type", elementTypeLabel(element)),
		propertyItem("X", fmt.Sprintf("%.1f mm", element.XMM)),
		propertyItem("Y", fmt.Sprintf("%.1f mm", element.YMM)),
		propertyItem("W", fmt.Sprintf("%.1f mm", element.WidthMM)),
		propertyItem("H", fmt.Sprintf("%.1f mm", element.HeightMM)),
		propertyItem("Rot", fmt.Sprintf("%d deg", element.Rotation)),
	)
	if element.Text != nil || element.QR != nil {
		lines = append(lines, helpItem("b", "name binding"), helpItem("!", "toggle required"))
	}
	for _, binding := range m.Bindings {
		if binding.ElementID == element.ID {
			lines = append(lines, propertyItem("Binding", binding.Name), propertyItem("Required", onOff(binding.Required)))
			break
		}
	}
	if element.Text != nil {
		lines = append(lines,
			"",
			propertyItem("Text", element.Text.Value),
			propertyItem("Font Size", fmt.Sprintf("%.0f", element.Text.FontSize)),
			propertyItem("Font", truncateText(m.selectedFontName(element.Text.FontPath), sidebarValueWidth("Font", width))),
			helpItem("F", "search fonts"),
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

func helpModalContent() []string {
	return []string{
		propertyTitleStyle.Render("Help"),
		"",
		helpRow("f", "Show focus hints for keyboard-only element selection"),
		helpRow("t", "Add a text box and select it"),
		helpRow("q", "Add a QR code and select it"),
		helpRow("r", "Rotate selected element"),
		helpRow("R", "Rotate the canvas"),
		helpRow("i / b / !", "Edit contents / name binding / toggle required"),
		helpRow("y / x / v / d", "Copy / cut / paste / duplicate selected component"),
		helpRow("z / Z / B", "Undo / redo / cycle redo branch"),
		helpRow("F", "Search fonts for the selected text box"),
		helpRow("e", "Export PNG to a chosen path"),
		helpRow("g", "Toggle visual grid"),
		helpRow("I", "Toggle inverted black/white colors"),
		helpRow("tab / ctrl+t", "Printer controls / switch designer and gallery"),
		helpRow("s", "Save current design preset"),
		helpRow("↑↓←→ / kjhl", "Move selected element by one canvas cell"),
		helpRow("shift+↑↓←→", "Move selected element by 5 canvas cells"),
		helpRow("4-9 then kjhl", "Move by a typed count of cells (e.g. 12l)"),
		helpRow("H / L", "Shrink / grow selected width"),
		helpRow("K / J", "Shrink / grow selected height"),
		helpRow("[ ] / { }", "Resize diagonally from the bottom-right"),
		helpRow("+ / -", "Increase / decrease selected text font size or QR size"),
		helpRow("p", "Open the native OS preview image"),
		helpRow("3", "Open the options menu"),
		helpRow("?", "Toggle this help popup"),
		helpRow("esc", "Close popup or clear selection"),
		helpRow("ctrl+c", "Quit immediately"),
	}
}

func menuModalContent(m Model) []string {
	autoInsert := "off"
	if m.AutoInsert {
		autoInsert = "on"
	}
	items := []string{
		"Auto Insert on Text/QR Creation: " + autoInsert,
		"Live Preview: " + livePreviewMenuState(m),
	}
	lines := []string{
		propertyTitleStyle.Render("Menu"),
		"",
	}
	for i, item := range items {
		prefix := "  "
		style := helpLabelStyle
		if i == m.MenuIndex {
			prefix = "> "
			style = propertySelectedStyle
		}
		lines = append(lines, style.Render(prefix+item))
	}
	lines = append(lines,
		"",
		mutedStyle.Render("↑/↓ or k/j move, enter selects, 1 or 2 leaves the menu"),
	)
	return lines
}

func livePreviewMenuState(m Model) string {
	switch m.LivePreviewMode {
	case config.LivePreviewTerminal:
		return "terminal image"
	case config.LivePreviewWindow:
		return "Preview window"
	case config.LivePreviewOff:
		return "off"
	}
	if m.Preview.Available == LivePreviewDisabled {
		return "auto (unavailable)"
	}
	return "auto (" + livePreviewProtocolLabel(m.Preview.Available) + ")"
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func helpRow(key, description string) string {
	return keyStyle.Render(fitLine(key, 14)) + helpLabelStyle.Render(description)
}

func footerLines(m Model, width int) []string {
	return panelLines("Keybinds", footerHelpLines(m, panelContentWidth(width)), width, layoutFooterHeight, false)
}

func footerHelpLines(m Model, width int) []string {
	if m.MenuOpen {
		return []string{
			truncateStyledLine(helpItem("↑/↓ k/j", "browse")+"  "+helpItem("enter", "select"), width),
		}
	}
	if m.Tab == tabGallery {
		return []string{
			truncateStyledLine(helpItem("↑/↓ k/j", "browse")+"  "+helpItem("←/→ h/l", "fold")+"  "+helpItem("enter", "open"), width),
			truncateStyledLine(helpItem("c", "command")+"  "+helpItem("d", "delete"), width),
		}
	}
	if m.SidebarFocused {
		return []string{
			truncateStyledLine(sidebarFooter(m), width),
			truncateStyledLine(helpItem("c", "connect")+"  "+helpItem("D", "disconnect")+"  "+helpItem("r", "rescan"), width),
		}
	}
	items := []string{}
	if element, ok := m.selectedElement(); ok {
		if element.Text != nil || element.QR != nil {
			items = append(items, helpItem("b", "binding"), helpItem("!", "required"), helpItem("+/-", "size"))
		}
		if element.Text != nil {
			items = append(items, helpItem("F", "fonts"))
		}
	}
	if m.Print.Session != nil {
		items = append(items, helpItem("P", "print"))
	}
	items = append(items,
		helpItem("f", "focus"),
		helpItem("t", "text"),
		helpItem("q", "QR"),
		helpItem("i", "edit"),
		helpItem("tab", "printer"),
		helpItem("↑↓←→/kjhl", "move"),
		helpItem("r", "rotate"),
		helpItem("R", "rotate canvas"),
		helpItem("HJKL", "resize"),
		helpItem("[]/{}", "resize diagonal"),
		helpItem("y/x/v/d", "copy/cut/paste/dup"),
		helpItem("z/Z", "undo/redo"),
		helpItem("g", "grid"),
		helpItem("I", "invert"),
		helpItem("s", "save"),
		helpItem("p", "preview"),
		helpItem("e", "export"),
		helpItem("?", "help"),
		helpItem("esc", "clear"),
	)
	return splitKeybindRows(items, width)
}

func splitKeybindRows(items []string, width int) []string {
	if len(items) == 0 {
		return []string{"", ""}
	}
	separator := "  "
	row := ""
	index := 0
	for ; index < len(items); index++ {
		candidate := items[index]
		if row != "" {
			candidate = row + separator + items[index]
		}
		if row != "" && lipgloss.Width(candidate) > width {
			break
		}
		row = candidate
	}
	if row == "" {
		row = items[0]
		index = 1
	}

	second := strings.Join(items[index:], separator)
	return []string{
		truncateStyledLine(row, width),
		truncateStyledLine(second, width),
	}
}

func propertyItem(label, value string) string {
	return propertyLabel(label) + " " + value
}

func propertyLabel(label string) string {
	return propertyLabelStyle.Render(label + ":")
}

func fontPickerLines(m Model, width int) []string {
	return fontPickerLinesWithLimit(m, width, fontPickerPageSize)
}

func fontPickerLinesWithLimit(m Model, width, itemLimit int) []string {
	if len(m.Fonts) == 0 {
		return []string{mutedStyle.Render("No fonts found")}
	}
	itemLimit = max(itemLimit, 1)
	indices := m.filteredFontIndices()
	query := truncateText(m.FontPickerQuery, max(width-len("Search: ")-1, 1))
	searchValue := query
	if m.FontPickerSearch {
		searchValue += string(promptCursorRune)
	}
	help := "browse: ↑/↓ or k/j"
	if m.FontPickerSearch {
		help = "type · ↑/↓ or ctrl+n/p"
	}
	lines := []string{
		propertyItem("Search", searchValue),
		mutedStyle.Render(help),
	}
	if len(indices) == 0 {
		return append(lines, mutedStyle.Render("No matches"))
	}
	selectedPosition := 0
	for i, index := range indices {
		if index == m.FontPickerIndex {
			selectedPosition = i
			break
		}
	}
	start := selectedPosition - itemLimit/2
	if start < 0 {
		start = 0
	}
	if start+itemLimit > len(indices) {
		start = max(len(indices)-itemLimit, 0)
	}
	end := min(start+itemLimit, len(indices))
	for i := start; i < end; i++ {
		fontIndex := indices[i]
		prefix := "  "
		style := mutedStyle
		if fontIndex == m.FontPickerIndex {
			prefix = "> "
			style = propertySelectedStyle
		}
		name := truncateText(m.Fonts[fontIndex].Name, max(width-2, 1))
		lines = append(lines, style.Render(prefix+name))
	}
	return lines
}

func (m Model) selectedFontName(fontPath string) string {
	fontPath = strings.TrimSpace(fontPath)
	for _, font := range m.Fonts {
		if strings.TrimSpace(font.Path) == fontPath {
			return font.Name
		}
	}
	return fontDisplayName(fontPath)
}

func connectHelp(m Model) string {
	if !m.SidebarFocused {
		return ""
	}
	if m.Print.Session == nil && m.Print.NewSession == nil {
		return ""
	}
	switch m.Connection {
	case ConnectionConnected:
		return helpItem("D", "disconnect")
	case ConnectionDisconnected:
		return helpItem("c", "reconnect")
	case ConnectionConnecting:
		return mutedStyle.Render("Connecting...")
	default:
		return helpItem("c", "connect")
	}
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
		return connectionDotStyle("215").Render("●") + helpLabelStyle.Render(" connecting")
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

func truncateStyledLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	cells := styledCells(s)
	if len(cells) <= width {
		return s
	}
	if width <= 3 {
		return strings.Join(cells[:width], "")
	}
	return strings.Join(cells[:width-3], "") + "..."
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

func drawPrintDirectionGuide(grid [][]rune, canvas Canvas, m Model) {
	if canvas.Width < 3 || canvas.Height < 3 {
		return
	}
	midX := canvas.Width / 2
	midY := canvas.Height / 2
	switch effectivePrintDirection(m) {
	case printDirectionUp:
		grid[0][midX] = '▲'
	case printDirectionRight:
		grid[midY][canvas.Width-1] = '▶'
	case printDirectionDown:
		grid[canvas.Height-1][midX] = '▼'
	case printDirectionLeft:
		grid[midY][0] = '◀'
	}
}

func drawPrintableAreaGuide(grid [][]rune, canvas Canvas, m Model) {
	if strings.EqualFold(m.Document.Shape, "round") {
		return
	}
	printableWidthMM := render.ModelPrintableWidthMM(m.Print.Model)
	if printableWidthMM <= 0 || printableWidthMM >= m.Document.WidthMM {
		if effectivePrintDirection(m).horizontal() && printableWidthMM > 0 && printableWidthMM < m.Document.HeightMM {
			drawHorizontalPrintableAreaGuide(grid, canvas, m, printableWidthMM)
		}
		return
	}
	if effectivePrintDirection(m).horizontal() {
		if printableWidthMM < m.Document.HeightMM {
			drawHorizontalPrintableAreaGuide(grid, canvas, m, printableWidthMM)
		}
		return
	}
	leftMM := (m.Document.WidthMM - printableWidthMM) / 2
	rightMM := leftMM + printableWidthMM
	leftX, _ := canvas.LabelToScreen(leftMM, 0)
	rightX, _ := canvas.LabelToScreen(rightMM, 0)
	left := clampInt(leftX-canvas.X, 1, max(canvas.Width-2, 1))
	right := clampInt(rightX-canvas.X, 1, max(canvas.Width-2, 1))
	if right <= left {
		return
	}
	grid[0][left] = '┬'
	grid[0][right] = '┬'
	grid[canvas.Height-1][left] = '┴'
	grid[canvas.Height-1][right] = '┴'
	for y := 1; y < canvas.Height-1; y++ {
		grid[y][left] = printableGuideRune
		grid[y][right] = printableGuideRune
	}
}

func drawHorizontalPrintableAreaGuide(grid [][]rune, canvas Canvas, m Model, printableWidthMM float64) {
	topMM := (m.Document.HeightMM - printableWidthMM) / 2
	bottomMM := topMM + printableWidthMM
	_, topY := canvas.LabelToScreen(0, topMM)
	_, bottomY := canvas.LabelToScreen(0, bottomMM)
	top := clampInt(topY-canvas.Y, 1, max(canvas.Height-2, 1))
	bottom := clampInt(bottomY-canvas.Y, 1, max(canvas.Height-2, 1))
	if bottom <= top {
		return
	}
	grid[top][0] = '├'
	grid[top][canvas.Width-1] = '┤'
	grid[bottom][0] = '├'
	grid[bottom][canvas.Width-1] = '┤'
	for x := 1; x < canvas.Width-1; x++ {
		grid[top][x] = printableGuideHorz
		grid[bottom][x] = printableGuideHorz
	}
}

type printDirection int

const (
	printDirectionUnknown printDirection = iota
	printDirectionUp
	printDirectionRight
	printDirectionDown
	printDirectionLeft
)

func (d printDirection) horizontal() bool {
	return d == printDirectionLeft || d == printDirectionRight
}

func effectivePrintDirection(m Model) printDirection {
	base := printDirectionUnknown
	switch printDirectionLabel(m.Print.Model) {
	case "bottom to top":
		base = printDirectionUp
	case "left to right":
		base = printDirectionRight
	}
	if base == printDirectionUnknown {
		return base
	}
	steps := normalizedTUIRotation(m.Document.Rotation) / 90
	return rotatePrintDirection(base, steps)
}

func rotatePrintDirection(direction printDirection, steps int) printDirection {
	if direction == printDirectionUnknown {
		return direction
	}
	directions := []printDirection{printDirectionUp, printDirectionRight, printDirectionDown, printDirectionLeft}
	index := int(direction) - int(printDirectionUp)
	return directions[(index+steps)%len(directions)]
}
