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
