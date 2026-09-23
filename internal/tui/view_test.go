package tui

import (
	"testing"

	"niimtui/internal/config"
)

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

func TestReflowCentersCanvasInPanel(t *testing.T) {
	m := NewModel(50, 50, "round", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()

	wantX := m.canvasPanelLeft() + (m.canvasPanelWidth()-m.Canvas.Width)/2
	if m.Canvas.X != wantX {
		t.Fatalf("canvas x = %d, want centered x %d", m.Canvas.X, wantX)
	}
}

func TestReflowCentersWideCanvasVertically(t *testing.T) {
	m := NewModel(80, 30, "rect", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()

	wantY := layoutBodyTop + (m.canvasPanelHeight()-m.Canvas.Height)/2
	if m.Canvas.Y != wantY {
		t.Fatalf("canvas y = %d, want centered y %d", m.Canvas.Y, wantY)
	}
}

func TestSwitchPresetUpdatesDocumentAndCanvas(t *testing.T) {
	presets := []config.LabelPreset{
		{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"},
		{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round"},
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{}, presets, "b1-50x30")
	m.Width = 160
	m.Height = 30
	m.reflow()

	if !m.switchPreset(1) {
		t.Fatal("switchPreset() = false, want true")
	}
	if m.Document.WidthMM != 50 || m.Document.HeightMM != 50 || m.Document.Shape != "round" {
		t.Fatalf("document = %.0fx%.0f %s, want 50x50 round", m.Document.WidthMM, m.Document.HeightMM, m.Document.Shape)
	}
	if m.Preset != 1 {
		t.Fatalf("preset index = %d, want 1", m.Preset)
	}
	wantX := m.canvasPanelLeft() + (m.canvasPanelWidth()-m.Canvas.Width)/2
	if m.Canvas.X != wantX {
		t.Fatalf("canvas x = %d, want centered x %d", m.Canvas.X, wantX)
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
