package render

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"niimcli/internal/label"
)

func TestRenderDocumentProducesPNG(t *testing.T) {
	doc := label.NewDocument(50, 30)
	if err := doc.AddElement(label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	if result.WidthPx != 400 || result.HeightPx != 240 {
		t.Fatalf("render size = %dx%d, want 400x240", result.WidthPx, result.HeightPx)
	}
	if len(result.PreviewPNG) == 0 {
		t.Fatal("RenderDocument() returned empty preview")
	}
	img, err := png.Decode(bytes.NewReader(result.PreviewPNG))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, 400, 240) {
		t.Fatalf("preview bounds = %v, want %v", img.Bounds(), image.Rect(0, 0, 400, 240))
	}
}

func TestRenderDocumentDrawsText(t *testing.T) {
	doc := label.NewDocument(40, 20)
	if err := doc.AddElement(label.NewTextElement("title", "Test", 2, 2, 20, 8, 18)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}

	blackPixels := 0
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			if gray.GrayAt(x, y).Y == 0 {
				blackPixels++
			}
		}
	}
	if blackPixels == 0 {
		t.Fatal("expected rendered text to produce black pixels")
	}
}

func TestLayoutTextWrapsWordsIntoMultipleLines(t *testing.T) {
	layout, err := LayoutText("Storage Box", 18, mmToPx(12))
	if err != nil {
		t.Fatalf("LayoutText() error = %v", err)
	}
	if len(layout.Lines) < 2 {
		t.Fatalf("line count = %d, want at least 2", len(layout.Lines))
	}
	if layout.BlockHeightPx <= layout.LineHeightPx {
		t.Fatalf("block height = %d, want greater than line height %d", layout.BlockHeightPx, layout.LineHeightPx)
	}
}

func TestRequiredTextHeightIncreasesWhenWidthShrinks(t *testing.T) {
	element := label.NewTextElement("title", "Storage Box 12", 0, 0, 20, 6, 18)
	wideHeight, err := RequiredTextHeightMM(element, 20)
	if err != nil {
		t.Fatalf("RequiredTextHeightMM() wide error = %v", err)
	}
	narrowHeight, err := RequiredTextHeightMM(element, 10)
	if err != nil {
		t.Fatalf("RequiredTextHeightMM() narrow error = %v", err)
	}
	if narrowHeight <= wideHeight {
		t.Fatalf("narrow height = %.2f, want greater than wide height %.2f", narrowHeight, wideHeight)
	}
}

func TestRenderDocumentLeavesTextTopPadding(t *testing.T) {
	element := label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)
	doc := label.NewDocument(50, 30)
	if err := doc.AddElement(element); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}

	left := mmToPx(element.XMM)
	right := mmToPx(element.XMM + element.WidthMM)
	top := mmToPx(element.YMM)
	paddingBottom := top + textPaddingPx()
	for y := top; y < paddingBottom; y++ {
		for x := left; x < right; x++ {
			if gray.GrayAt(x, y).Y == 0 {
				t.Fatalf("found black pixel in top padding at (%d,%d)", x, y)
			}
		}
	}
}
