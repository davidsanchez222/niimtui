package niimbot

import (
	"fmt"
	"image"
	"math"

	"niimtui/internal/render"
)

type D110Job struct {
	Density   byte
	LabelType byte
	Rows      [][]byte
	WidthPx   int
	HeightPx  int
}

func PrepareD110Job(result render.Result, density int) (D110Job, error) {
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		return D110Job{}, fmt.Errorf("expected grayscale rendered image")
	}

	oriented := gray
	if gray.Bounds().Dx() > gray.Bounds().Dy() {
		oriented = rotateGray90CW(gray)
	}

	widthPx := oriented.Bounds().Dx()
	heightPx := oriented.Bounds().Dy()
	if widthPx%8 != 0 {
		return D110Job{}, fmt.Errorf("d110 oriented image width must be a multiple of 8 pixels")
	}

	rows := make([][]byte, 0, heightPx)
	bytesPerRow := int(math.Ceil(float64(widthPx) / 8.0))
	for y := 0; y < heightPx; y++ {
		row := make([]byte, bytesPerRow)
		for x := 0; x < widthPx; x++ {
			if isBlack(oriented.GrayAt(x, y)) {
				row[x/8] |= 1 << (7 - uint(x%8))
			}
		}
		rows = append(rows, row)
	}

	if density < 0 {
		density = 0
	}
	if density > 5 {
		density = 5
	}

	return D110Job{
		Density:   byte(density),
		LabelType: 0x01,
		Rows:      rows,
		WidthPx:   widthPx,
		HeightPx:  heightPx,
	}, nil
}

func rotateGray90CW(src *image.Gray) *image.Gray {
	sb := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, sb.Dy(), sb.Dx()))
	for y := sb.Min.Y; y < sb.Max.Y; y++ {
		for x := sb.Min.X; x < sb.Max.X; x++ {
			v := src.GrayAt(x, y)
			sx := x - sb.Min.X
			sy := y - sb.Min.Y
			dst.SetGray(sb.Dy()-1-sy, sx, v)
		}
	}
	return dst
}
