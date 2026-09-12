package niimbot

import (
	"image"
	"image/color"
	"testing"

	"niimtui/internal/render"
)

func TestPrepareD110JobRotatesLandscapeToNinetySixWide(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 320, 96))
	fillWhite(img)
	for x := 0; x < 8; x++ {
		img.SetGray(x, 95, color.Gray{Y: 0})
	}

	job, err := PrepareD110Job(render.Result{Image: img, WidthPx: 320, HeightPx: 96}, 3)
	if err != nil {
		t.Fatalf("PrepareD110Job() error = %v", err)
	}
	if job.WidthPx != 96 || job.HeightPx != 320 {
		t.Fatalf("dimensions = %dx%d, want 96x320", job.WidthPx, job.HeightPx)
	}
	if len(job.Rows) != 320 {
		t.Fatalf("rows = %d, want 320", len(job.Rows))
	}
	if len(job.Rows[0]) != 12 {
		t.Fatalf("row bytes = %d, want 12", len(job.Rows[0]))
	}
	for row := 0; row < 8; row++ {
		if got, want := job.Rows[row][0], byte(0x80); got != want {
			t.Fatalf("row %d first byte = 0x%02x, want 0x80", row, got)
		}
		for i := 1; i < len(job.Rows[row]); i++ {
			if job.Rows[row][i] != 0x00 {
				t.Fatalf("row %d byte %d = 0x%02x, want 0x00", row, i, job.Rows[row][i])
			}
		}
	}
}

func TestPrepareRasterJobPacksRowsLeftToRight(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 2))
	fillWhite(img)
	for x := 0; x < 8; x++ {
		img.SetGray(x, 0, color.Gray{Y: 0})
	}
	img.SetGray(15, 1, color.Gray{Y: 0})

	job, err := PrepareRasterJob(render.Result{Image: img, WidthPx: 16, HeightPx: 2})
	if err != nil {
		t.Fatalf("PrepareRasterJob() error = %v", err)
	}
	if len(job.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(job.Rows))
	}
	if got, want := job.Rows[0], []byte{0xff, 0x00}; !equalBytes(got, want) {
		t.Fatalf("row0 = %v, want %v", got, want)
	}
	if got, want := job.Rows[1], []byte{0x00, 0x01}; !equalBytes(got, want) {
		t.Fatalf("row1 = %v, want %v", got, want)
	}
}

func fillWhite(img *image.Gray) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
