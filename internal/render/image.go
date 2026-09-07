package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

func PNGFile(path string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return Result{}, fmt.Errorf("decode png: %w", err)
	}
	return Image(img)
}

func Image(img image.Image) (Result, error) {
	if img == nil {
		return Result{}, fmt.Errorf("nil image")
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return Result{}, fmt.Errorf("invalid image dimensions")
	}

	gray := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(gray, gray.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(gray, gray.Bounds(), img, bounds.Min, draw.Src)
	thresholdToMonochrome(gray)

	var preview bytes.Buffer
	if err := png.Encode(&preview, gray); err != nil {
		return Result{}, fmt.Errorf("encode preview: %w", err)
	}

	return Result{
		Image:        gray,
		WidthPx:      gray.Bounds().Dx(),
		HeightPx:     gray.Bounds().Dy(),
		PrintablePx:  gray.Bounds(),
		Shape:        "rect",
		PreviewPNG:   preview.Bytes(),
		PreviewBytes: preview.Len(),
	}, nil
}
