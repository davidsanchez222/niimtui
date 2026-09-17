package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
)

func CalibrationLabel(widthMM, heightMM float64, shape string) (Result, error) {
	if widthMM <= 0 || heightMM <= 0 {
		return Result{}, fmt.Errorf("invalid calibration dimensions")
	}
	widthPx := mmToPx(widthMM)
	heightPx := mmToPx(heightMM)
	if widthPx <= 0 || heightPx <= 0 {
		return Result{}, fmt.Errorf("invalid calibration output size")
	}

	canvas := image.NewGray(image.Rect(0, 0, widthPx, heightPx))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	drawCalibrationPattern(canvas)
	thresholdToMonochrome(canvas)
	if strings.EqualFold(shape, "round") {
		maskRound(canvas)
	}

	var preview bytes.Buffer
	if err := png.Encode(&preview, canvas); err != nil {
		return Result{}, fmt.Errorf("encode preview: %w", err)
	}
	return Result{
		Image:        canvas,
		WidthPx:      widthPx,
		HeightPx:     heightPx,
		PrintablePx:  canvas.Bounds(),
		Shape:        shape,
		PreviewPNG:   preview.Bytes(),
		PreviewBytes: preview.Len(),
	}, nil
}

func drawCalibrationPattern(img *image.Gray) {
	b := img.Bounds()
	black := color.Gray{Y: 0}
	line := max(2, min(b.Dx(), b.Dy())/120)
	inset := max(mmToPx(3), min(b.Dx(), b.Dy())/16)

	fillRect(img, image.Rect(b.Min.X+inset, b.Min.Y+inset, b.Max.X-inset, b.Min.Y+inset+line), black)
	fillRect(img, image.Rect(b.Min.X+inset, b.Max.Y-inset-line, b.Max.X-inset, b.Max.Y-inset), black)
	fillRect(img, image.Rect(b.Min.X+inset, b.Min.Y+inset, b.Min.X+inset+line, b.Max.Y-inset), black)
	fillRect(img, image.Rect(b.Max.X-inset-line, b.Min.Y+inset, b.Max.X-inset, b.Max.Y-inset), black)

	centerX := b.Min.X + b.Dx()/2
	centerY := b.Min.Y + b.Dy()/2
	fillRect(img, image.Rect(centerX-line/2, b.Min.Y+inset, centerX+line-line/2, b.Max.Y-inset), black)
	fillRect(img, image.Rect(b.Min.X+inset, centerY-line/2, b.Max.X-inset, centerY+line-line/2), black)

	for x := b.Min.X + inset; x < b.Max.X-inset; x += mmToPx(10) {
		fillRect(img, image.Rect(x, b.Min.Y+inset, x+line, b.Min.Y+inset+mmToPx(3)), black)
		fillRect(img, image.Rect(x, b.Max.Y-inset-mmToPx(3), x+line, b.Max.Y-inset), black)
	}
	for y := b.Min.Y + inset; y < b.Max.Y-inset; y += mmToPx(10) {
		fillRect(img, image.Rect(b.Min.X+inset, y, b.Min.X+inset+mmToPx(3), y+line), black)
		fillRect(img, image.Rect(b.Max.X-inset-mmToPx(3), y, b.Max.X-inset, y+line), black)
	}
}

func fillRect(dst draw.Image, rect image.Rectangle, c color.Color) {
	draw.Draw(dst, rect.Intersect(dst.Bounds()), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
