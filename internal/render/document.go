package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"niimcli/internal/label"
)

var regularFont *opentype.Font

func init() {
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(fmt.Sprintf("parse embedded font: %v", err))
	}
	regularFont = parsed
}

func RenderDocument(doc label.Document) (Result, error) {
	if doc.WidthMM <= 0 || doc.HeightMM <= 0 {
		return Result{}, fmt.Errorf("invalid document dimensions")
	}

	widthPx := mmToPx(doc.WidthMM)
	heightPx := mmToPx(doc.HeightMM)
	if widthPx <= 0 || heightPx <= 0 {
		return Result{}, fmt.Errorf("invalid output size")
	}

	canvas := image.NewGray(image.Rect(0, 0, widthPx, heightPx))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	for _, element := range doc.Elements {
		if err := drawDocumentElement(canvas, element); err != nil {
			return Result{}, err
		}
	}

	thresholdToMonochrome(canvas)
	if strings.EqualFold(doc.Shape, "round") {
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
		Shape:        doc.Shape,
		PreviewPNG:   preview.Bytes(),
		PreviewBytes: preview.Len(),
	}, nil
}

func drawDocumentElement(dst draw.Image, element label.Element) error {
	switch element.Type {
	case label.ElementText:
		return drawTextElement(dst, element)
	default:
		return fmt.Errorf("unsupported element type %q", element.Type)
	}
}

func drawTextElement(dst draw.Image, element label.Element) error {
	if element.Text == nil {
		return nil
	}
	rect := image.Rect(
		mmToPx(element.XMM),
		mmToPx(element.YMM),
		mmToPx(element.XMM+element.WidthMM),
		mmToPx(element.YMM+element.HeightMM),
	)
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return nil
	}

	face, err := opentype.NewFace(regularFont, &opentype.FaceOptions{
		Size:    maxFloat(element.Text.FontSize, 8),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return fmt.Errorf("create font face: %w", err)
	}
	defer face.Close()

	line := fitText(strings.TrimSpace(element.Text.Value), face, rect.Dx())
	if line == "" {
		return nil
	}

	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	baselineY := rect.Min.Y + max(0, (rect.Dy()-lineHeight)/2) + metrics.Ascent.Ceil()
	textWidth := font.MeasureString(face, line).Ceil()
	textX := rect.Min.X + max(0, (rect.Dx()-textWidth)/2)

	d := font.Drawer{
		Dst:  dst,
		Src:  image.Black,
		Face: face,
		Dot:  fixed.P(textX, baselineY),
	}
	d.DrawString(line)
	return nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
