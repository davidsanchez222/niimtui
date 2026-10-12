package render

import (
	"bytes"
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

func TestFitToPrinterWidthScalesB1ImagesToPrintableWidth(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 400, 240))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	img.SetGray(0, 120, color.Gray{Y: 0})

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
	fitted := result.Image.(*image.Gray)
	foundLeftEdge := false
	for y := fitted.Bounds().Min.Y; y < fitted.Bounds().Max.Y; y++ {
		if fitted.GrayAt(0, y).Y == 0 {
			foundLeftEdge = true
			break
		}
	}
	if !foundLeftEdge {
		t.Fatal("left-edge source content was not preserved after fitting")
	}
}

func TestFitToPrinterWidthKeepsB150x80FullLength(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 400, 640))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	img.SetGray(0, 0, color.Gray{Y: 0})
	img.SetGray(399, 639, color.Gray{Y: 0})

	result, err := FitToPrinterWidth(Result{Image: img, WidthPx: 400, HeightPx: 640, PrintablePx: img.Bounds(), Shape: "rect"}, "B1")
	if err != nil {
		t.Fatalf("FitToPrinterWidth() error = %v", err)
	}
	if result.WidthPx != 384 || result.HeightPx != 640 {
		t.Fatalf("render size = %dx%d, want 384x640", result.WidthPx, result.HeightPx)
	}
	for _, point := range []image.Point{{X: 0, Y: 0}, {X: 383, Y: 639}} {
		if got := result.Image.(*image.Gray).GrayAt(point.X, point.Y).Y; got != 0 {
			t.Errorf("print pixel at %v = %d, want black", point, got)
		}
	}
	preview, err := png.Decode(bytes.NewReader(result.PreviewPNG))
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	for _, point := range []image.Point{{X: 0, Y: 0}, {X: 383, Y: 639}} {
		if got := color.GrayModel.Convert(preview.At(point.X, point.Y)).(color.Gray).Y; got != 0 {
			t.Errorf("preview pixel at %v = %d, want black", point, got)
		}
	}
}

func TestFitToPrinterWidthStillFitsOtherB1RollsProportionally(t *testing.T) {
	tests := []struct {
		name   string
		shape  string
		height int
	}{
		{name: "round 50x50", shape: "round", height: 400},
		{name: "rectangular 50x70", shape: "rect", height: 560},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewGray(image.Rect(0, 0, 400, tt.height))
			result, err := FitToPrinterWidth(Result{Image: img, Shape: tt.shape}, "B1")
			if err != nil {
				t.Fatalf("FitToPrinterWidth() error = %v", err)
			}
			fitted := result.Image.(*image.Gray)
			if fitted.GrayAt(0, 0).Y != 255 || fitted.GrayAt(0, tt.height/2).Y != 0 || fitted.GrayAt(0, tt.height-1).Y != 255 {
				t.Fatal("other B1 rolls should retain their centered, proportional fit")
			}
		})
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
