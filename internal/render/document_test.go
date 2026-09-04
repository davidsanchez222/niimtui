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
