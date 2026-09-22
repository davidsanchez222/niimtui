package tui

import "testing"

func TestDrawRoundGuideUsesPrintableGuideRune(t *testing.T) {
	canvas := Canvas{Width: 32, Height: 16, LabelWidthMM: 50, LabelHeightMM: 50}
	grid := newTestGrid(canvas)

	drawRoundGuide(grid, canvas)

	guideCount := 0
	for y, row := range grid {
		for x, r := range row {
			switch {
			case r == printableGuideRune:
				guideCount++
			case r == '·':
				t.Fatalf("round guide at %d,%d uses middle dot, want printable guide rune", x, y)
			case r >= 0x2800 && r <= 0x28ff:
				t.Fatalf("round guide at %d,%d uses braille rune %q", x, y, r)
			}
		}
	}
	if guideCount == 0 {
		t.Fatal("round guide did not draw printable guide runes")
	}
}

func TestDrawPrintableAreaGuideSkipsRoundLabels(t *testing.T) {
	canvas := newCanvas(80, 24, 50, 50)
	m := NewModel(50, 50, "round", "", PrintConfig{Model: "B1"})
	grid := newTestGrid(canvas)

	drawPrintableAreaGuide(grid, canvas, m)

	if guideCount := countNonZeroRunes(grid); guideCount != 0 {
		t.Fatalf("round printable area guide drew %d cells, want none", guideCount)
	}

	m.Document.Shape = "rect"
	drawPrintableAreaGuide(grid, canvas, m)
	if guideCount := countNonZeroRunes(grid); guideCount == 0 {
		t.Fatal("rect printable area guide did not draw any cells")
	}
}

func newTestGrid(canvas Canvas) [][]rune {
	grid := make([][]rune, canvas.Height)
	for y := range grid {
		grid[y] = make([]rune, canvas.Width)
	}
	return grid
}

func countNonZeroRunes(grid [][]rune) int {
	count := 0
	for _, row := range grid {
		for _, r := range row {
			if r != 0 {
				count++
			}
		}
	}
	return count
}
