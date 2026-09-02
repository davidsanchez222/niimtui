package tui

import "math"

// Approximate cell width/height ratio for the current terminal font.
// A visually square shape needs more columns than rows.
const terminalCellWidthToHeightRatio = 0.43

type Canvas struct {
	X int
	Y int

	Width  int
	Height int

	LabelWidthMM  float64
	LabelHeightMM float64

	CellsPerMMX float64
	CellsPerMMY float64
}

func newCanvas(terminalWidth, terminalHeight int, labelWidthMM, labelHeightMM float64) Canvas {
	const minCanvasWidth = 12
	const minCanvasHeight = 6

	availableWidth := max(terminalWidth, minCanvasWidth)
	availableHeight := max(terminalHeight, minCanvasHeight)

	innerWidth := max(availableWidth-2, 1)
	innerHeight := max(availableHeight-2, 1)

	// Compare available width in visual units, not raw columns.
	scaleX := (float64(innerWidth) * terminalCellWidthToHeightRatio) / labelWidthMM
	scaleY := float64(innerHeight) / labelHeightMM
	scale := math.Min(scaleX, scaleY)
	if scale <= 0 {
		scale = 1
	}

	renderWidth := max(int(math.Round((labelWidthMM*scale)/terminalCellWidthToHeightRatio)), 1)
	renderHeight := max(int(math.Round(labelHeightMM*scale)), 1)
	renderWidth = min(renderWidth, innerWidth)
	renderHeight = min(renderHeight, innerHeight)

	return Canvas{
		Width:         renderWidth + 2,
		Height:        renderHeight + 2,
		LabelWidthMM:  labelWidthMM,
		LabelHeightMM: labelHeightMM,
		CellsPerMMX:   float64(renderWidth) / labelWidthMM,
		CellsPerMMY:   float64(renderHeight) / labelHeightMM,
	}
}

func (c Canvas) LabelToScreen(xMM, yMM float64) (int, int) {
	x := c.X + 1 + int(math.Round(xMM*c.CellsPerMMX))
	y := c.Y + 1 + int(math.Round(yMM*c.CellsPerMMY))
	return x, y
}

func (c Canvas) ScreenToLabel(x, y int) (float64, float64) {
	labelX := float64(x-c.X-1) / c.CellsPerMMX
	labelY := float64(y-c.Y-1) / c.CellsPerMMY
	return labelX, labelY
}

func (c Canvas) CellsToMM(dx, dy int) (float64, float64) {
	return float64(dx) / c.CellsPerMMX, float64(dy) / c.CellsPerMMY
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
