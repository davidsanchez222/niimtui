package niimbot

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"niimtui/internal/render"
)

func TestPrepareRasterJobRotatesLandscapeToNinetySixWide(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 320, 96))
	fillWhite(img)
	for x := 0; x < 8; x++ {
		img.SetGray(x, 95, color.Gray{Y: 0})
	}

	job, err := PrepareRasterJob(render.Result{Image: img}, RasterRotateLandscape)
	if err != nil {
		t.Fatal(err)
	}
	if job.WidthPx != 96 || job.HeightPx != 320 || len(job.Rows) != 320 {
		t.Fatalf("dimensions = %dx%d, rows = %d; want 96x320, 320 rows", job.WidthPx, job.HeightPx, len(job.Rows))
	}
	want := append([]byte{0x80}, make([]byte, 11)...)
	for row := 0; row < 8; row++ {
		if !slices.Equal(job.Rows[row], want) {
			t.Fatalf("row %d = %x, want %x", row, job.Rows[row], want)
		}
	}
}

func TestPrepareRasterJobKeepsPortraitOrientation(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 96, 320))
	fillWhite(img)
	img.SetGray(0, 0, color.Gray{Y: 0})
	job, err := PrepareRasterJob(render.Result{Image: img}, RasterRotateLandscape)
	if err != nil {
		t.Fatal(err)
	}
	if job.WidthPx != 96 || job.HeightPx != 320 || job.Rows[0][0] != 0x80 {
		t.Fatalf("portrait image was changed: dimensions %dx%d, first byte %02x", job.WidthPx, job.HeightPx, job.Rows[0][0])
	}
}

func TestPrepareRasterJobPacksRowsLeftToRight(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 2))
	fillWhite(img)
	for x := 0; x < 8; x++ {
		img.SetGray(x, 0, color.Gray{Y: 0})
	}
	img.SetGray(15, 1, color.Gray{Y: 0})

	job, err := PrepareRasterJob(render.Result{Image: img}, RasterAsRendered)
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Rows) != 2 || !slices.Equal(job.Rows[0], []byte{0xff, 0}) || !slices.Equal(job.Rows[1], []byte{0, 1}) {
		t.Fatalf("rows = %v, want [ff00 0001]", job.Rows)
	}
}

func TestPrepareRasterJobRequiresByteAlignedRotatedWidth(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 32, 7))
	if _, err := PrepareRasterJob(render.Result{Image: img}, RasterRotateLandscape); err == nil {
		t.Fatal("expected error for oriented width of seven pixels")
	}
	unaligned := image.NewGray(image.Rect(0, 0, 7, 32))
	if _, err := PrepareRasterJob(render.Result{Image: unaligned}, RasterAsRendered); err != nil {
		t.Fatalf("unaligned width should be valid without rotation: %v", err)
	}
}

func fillWhite(img *image.Gray) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
}
