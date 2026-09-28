package niimbot

import (
	"fmt"
	"image"

	"niimtui/internal/render"
)

type RasterJob struct {
	Rows     [][]byte
	WidthPx  int
	HeightPx int
}

// RasterOrientation selects how a rendered image is laid out on the printhead.
type RasterOrientation uint8

const (
	RasterAsRendered RasterOrientation = iota
	// RasterRotateLandscape leaves already-portrait images unchanged.
	RasterRotateLandscape
)

func PrepareRasterJob(result render.Result, orientation RasterOrientation) (RasterJob, error) {
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		return RasterJob{}, fmt.Errorf("expected grayscale rendered image")
	}
	if orientation == RasterRotateLandscape && gray.Bounds().Dx() > gray.Bounds().Dy() {
		gray = rotateGray90CW(gray)
	}
	widthPx := gray.Bounds().Dx()
	heightPx := gray.Bounds().Dy()
	if orientation == RasterRotateLandscape && widthPx%8 != 0 {
		return RasterJob{}, fmt.Errorf("oriented image width must be a multiple of 8 pixels")
	}
	bytesPerRow := (widthPx + 7) / 8
	rows := make([][]byte, 0, heightPx)
	for y := 0; y < heightPx; y++ {
		row := make([]byte, bytesPerRow)
		for x := 0; x < widthPx; x++ {
			if gray.GrayAt(x, y).Y < 128 {
				row[x/8] |= 1 << (7 - uint(x%8))
			}
		}
		rows = append(rows, row)
	}
	return RasterJob{Rows: rows, WidthPx: widthPx, HeightPx: heightPx}, nil
}

func rotateGray90CW(src *image.Gray) *image.Gray {
	sb := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, sb.Dy(), sb.Dx()))
	for y := sb.Min.Y; y < sb.Max.Y; y++ {
		for x := sb.Min.X; x < sb.Max.X; x++ {
			sx := x - sb.Min.X
			sy := y - sb.Min.Y
			dst.SetGray(sb.Dy()-1-sy, sx, src.GrayAt(x, y))
		}
	}
	return dst
}
