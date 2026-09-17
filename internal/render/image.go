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

func FitToPrinterWidth(result Result, model string) (Result, error) {
	maxWidth := maxPrintableWidthPx(model)
	if maxWidth == 0 || result.Image == nil {
		return result, nil
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		return Result{}, fmt.Errorf("expected grayscale rendered image")
	}
	bounds := gray.Bounds()
	if bounds.Dx() <= maxWidth {
		return result, nil
	}

	cropMinX := bounds.Min.X + (bounds.Dx()-maxWidth)/2
	crop := image.Rect(cropMinX, bounds.Min.Y, cropMinX+maxWidth, bounds.Max.Y)
	fitted := image.NewGray(image.Rect(0, 0, maxWidth, bounds.Dy()))
	draw.Draw(fitted, fitted.Bounds(), gray, crop.Min, draw.Src)

	var preview bytes.Buffer
	if err := png.Encode(&preview, fitted); err != nil {
		return Result{}, fmt.Errorf("encode preview: %w", err)
	}
	result.Image = fitted
	result.WidthPx = fitted.Bounds().Dx()
	result.HeightPx = fitted.Bounds().Dy()
	result.PrintablePx = fitted.Bounds()
	result.PreviewPNG = preview.Bytes()
	result.PreviewBytes = preview.Len()
	return result, nil
}

func ApplyPrintOffset(result Result, offsetXMM, offsetYMM float64) (Result, error) {
	if offsetXMM == 0 && offsetYMM == 0 {
		return result, nil
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		return Result{}, fmt.Errorf("expected grayscale rendered image")
	}
	bounds := gray.Bounds()
	shiftX := mmToPx(offsetXMM)
	shiftY := mmToPx(offsetYMM)

	shifted := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(shifted, shifted.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(shifted, bounds.Add(image.Pt(shiftX, shiftY)), gray, bounds.Min, draw.Src)
	thresholdToMonochrome(shifted)

	var preview bytes.Buffer
	if err := png.Encode(&preview, shifted); err != nil {
		return Result{}, fmt.Errorf("encode preview: %w", err)
	}
	result.Image = shifted
	result.WidthPx = shifted.Bounds().Dx()
	result.HeightPx = shifted.Bounds().Dy()
	result.PrintablePx = shifted.Bounds()
	result.PreviewPNG = preview.Bytes()
	result.PreviewBytes = preview.Len()
	return result, nil
}

func ModelPrintOffsetMM(model string, offsetXMM, offsetYMM float64) (float64, float64) {
	return offsetXMM, offsetYMM
}

func ModelPrintableWidthMM(model string) float64 {
	maxWidth := maxPrintableWidthPx(model)
	if maxWidth == 0 {
		return 0
	}
	return float64(maxWidth) / dotsPerMM
}
