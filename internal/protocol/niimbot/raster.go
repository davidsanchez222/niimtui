package niimbot

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"niimtui/internal/render"
)

type RasterJob struct {
	Rows     [][]byte
	WidthPx  int
	HeightPx int
}

func PrepareRasterJob(result render.Result) (RasterJob, error) {
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		return RasterJob{}, fmt.Errorf("expected grayscale rendered image")
	}
	widthPx := gray.Bounds().Dx()
	heightPx := gray.Bounds().Dy()
	bytesPerRow := int(math.Ceil(float64(widthPx) / 8.0))
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

func isBlack(c color.Gray) bool {
	return c.Y < 128
}
