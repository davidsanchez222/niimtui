package tui

import (
	"testing"

	"niimcli/internal/label"
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
