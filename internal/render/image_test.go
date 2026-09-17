package render

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPNGFileLoadsMonochromeRenderResult(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 8))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	img.SetGray(2, 2, color.Gray{Y: 0})

	path := filepath.Join(t.TempDir(), "label.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	result, err := PNGFile(path)
	if err != nil {
		t.Fatalf("PNGFile() error = %v", err)
	}
	if result.WidthPx != 16 || result.HeightPx != 8 {
		t.Fatalf("render size = %dx%d, want 16x8", result.WidthPx, result.HeightPx)
	}
	if len(result.PreviewPNG) == 0 {
		t.Fatal("PNGFile() returned empty preview")
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("image type = %T, want *image.Gray", result.Image)
	}
	if gray.GrayAt(2, 2).Y != 0 {
		t.Fatalf("pixel = %d, want black", gray.GrayAt(2, 2).Y)
	}
}

func TestFitToPrinterWidthCropsB1ImagesToPrintableWidth(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 400, 240))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	img.SetGray(399, 239, color.Gray{Y: 0})

	result, err := FitToPrinterWidth(Result{Image: img, WidthPx: 400, HeightPx: 240, PrintablePx: img.Bounds()}, "B1")
	if err != nil {
		t.Fatalf("FitToPrinterWidth() error = %v", err)
	}
	if result.WidthPx != 384 {
		t.Fatalf("width = %d, want 384", result.WidthPx)
	}
	if result.HeightPx != 240 {
		t.Fatalf("height = %d, want 240", result.HeightPx)
	}
	if len(result.PreviewPNG) == 0 {
		t.Fatal("FitToPrinterWidth() returned empty preview")
	}
}

func TestApplyPrintOffsetShiftsContentWithinCanvas(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 40, 24))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	img.SetGray(16, 8, color.Gray{Y: 0})

	result, err := ApplyPrintOffset(Result{Image: img, WidthPx: 40, HeightPx: 24, PrintablePx: img.Bounds()}, -1, 1)
	if err != nil {
		t.Fatalf("ApplyPrintOffset() error = %v", err)
	}
	shifted := result.Image.(*image.Gray)
	if shifted.GrayAt(8, 16).Y != 0 {
		t.Fatalf("shifted pixel = %d, want black", shifted.GrayAt(8, 16).Y)
	}
	if shifted.GrayAt(16, 8).Y != 255 {
		t.Fatalf("original pixel = %d, want white", shifted.GrayAt(16, 8).Y)
	}
}

func TestModelPrintOffsetUsesExplicitOffsetsOnly(t *testing.T) {
	x, y := ModelPrintOffsetMM("B1", 0.25, -0.5)
	if x != 0.25 || y != -0.5 {
		t.Fatalf("B1 offset = %.2f, %.2f; want 0.25, -0.50", x, y)
	}
	x, y = ModelPrintOffsetMM("D110", 0.25, -0.5)
	if x != 0.25 || y != -0.5 {
		t.Fatalf("D110 offset = %.2f, %.2f; want 0.25, -0.50", x, y)
	}
}
