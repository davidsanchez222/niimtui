package tui

import (
	"testing"

	"niimtui/internal/label"
)

func TestAutoFitTextElementGrowsHeightForWrappedText(t *testing.T) {
	element := label.NewTextElement("title", "Storage Box 12", 0, 0, 10, 4, 18)
	autoFitTextElement(&element)
	if element.HeightMM <= 4 {
		t.Fatalf("height = %.2f, want greater than 4", element.HeightMM)
	}
}

func TestResizeElementKeepsWrappedTextInsideHeight(t *testing.T) {
	original := label.NewTextElement("title", "Storage Box 12", 0, 0, 12, 8, 18)
	updated := resizeElement(original, HandleBottom, 0, -20)
	if updated.HeightMM >= original.HeightMM && updated.HeightMM <= minElementHeightMM {
		t.Fatalf("height = %.2f, expected wrapped-text minimum larger than generic minimum", updated.HeightMM)
	}
	_, minHeight := minimumElementSize(updated)
	if updated.HeightMM < minHeight {
		t.Fatalf("height = %.2f, min height = %.2f", updated.HeightMM, minHeight)
	}
}

func TestNewModelAppliesFontPathToInitialAndAddedText(t *testing.T) {
	m := NewModel(50, 30, "rect", "/tmp/example.ttf", PrintConfig{})
	initial, ok := m.selectedElement()
	if !ok || initial.Text == nil {
		t.Fatal("expected initial selected text element")
	}
	if initial.Text.FontPath != "/tmp/example.ttf" {
		t.Fatalf("initial font path = %q, want custom path", initial.Text.FontPath)
	}

	m.addTextElement()
	added, ok := m.selectedElement()
	if !ok || added.Text == nil {
		t.Fatal("expected added selected text element")
	}
	if added.Text.FontPath != "/tmp/example.ttf" {
		t.Fatalf("added font path = %q, want custom path", added.Text.FontPath)
	}
}
