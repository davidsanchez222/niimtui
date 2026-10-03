package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func moveTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(80, 40, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = 200, 60, true
	m.reflow()
	m.addTextElement()
	m.applyTextBuffer("x")
	m.commitHistory("set text")
	m.EditingText = false
	return m
}

func pressKey(m Model, msg tea.KeyMsg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func pressRunes(m Model, keys string) Model {
	for _, r := range keys {
		m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func selectedLeft(t *testing.T, m Model) int {
	t.Helper()
	element, ok := m.selectedElement()
	if !ok {
		t.Fatal("no selected element")
	}
	return m.elementScreenRect(element).left
}

func TestCountPrefixMovesByCount(t *testing.T) {
	// Models share their document's element slice, so each move needs a fresh model.
	m := moveTestModel(t)
	start := selectedLeft(t, m)
	if got := selectedLeft(t, pressRunes(m, "l")) - start; got != 1 {
		t.Fatalf("l moved %d cells, want 1", got)
	}
	m = moveTestModel(t)
	seven := pressRunes(m, "7l")
	if got := selectedLeft(t, seven) - start; got != 7 {
		t.Fatalf("7l moved %d cells, want 7", got)
	}
	if seven.Count != 0 {
		t.Fatalf("count = %d after move, want cleared", seven.Count)
	}
}

func TestCountPrefixAllowsAnyDigitOnceStarted(t *testing.T) {
	m := moveTestModel(t)
	m = pressRunes(m, "512")
	if m.Count != 51 {
		// 5, 1 -> 51, 2 -> 99 clamp is only reached at 512 > 99.
		if m.Count != 99 {
			t.Fatalf("count = %d, want 99 clamp", m.Count)
		}
	}
	if m.Tab != tabDesigner {
		t.Fatalf("tab = %v, digits 1/2 must extend a pending count, not switch tabs", m.Tab)
	}
}

func TestCountPrefixClearedByOtherKey(t *testing.T) {
	m := moveTestModel(t)
	m = pressRunes(m, "9")
	if m.Count != 9 {
		t.Fatalf("count = %d, want 9", m.Count)
	}
	m = pressRunes(m, "g")
	if m.Count != 0 {
		t.Fatalf("count = %d after unrelated key, want cleared", m.Count)
	}
}

func TestDigitsStillSwitchTabsWithoutCount(t *testing.T) {
	m := moveTestModel(t)
	m = pressRunes(m, "2")
	if m.Tab != tabGallery {
		t.Fatalf("tab = %v, want gallery", m.Tab)
	}
}

func TestShiftArrowMovesFiveCells(t *testing.T) {
	m := moveTestModel(t)
	start := selectedLeft(t, m)
	fast := pressKey(m, tea.KeyMsg{Type: tea.KeyShiftRight})
	if got := selectedLeft(t, fast) - start; got != fastMoveCells {
		t.Fatalf("shift+right moved %d cells, want %d", got, fastMoveCells)
	}
}
