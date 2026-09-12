package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"niimtui/internal/label"
)

var (
	regularFont *opentype.Font
	textFont    *opentype.Font
)

type documentRenderOptions struct {
	threshold uint8
}

func init() {
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(fmt.Sprintf("parse embedded font: %v", err))
	}
	regularFont = parsed

	parsed, err = opentype.Parse(gomedium.TTF)
	if err != nil {
		panic(fmt.Sprintf("parse embedded text font: %v", err))
	}
	textFont = parsed
}

func RenderDocument(doc label.Document) (Result, error) {
	return renderDocumentWithOptions(doc, documentRenderOptions{threshold: textThreshold})
}

func RenderDocumentForTUI(doc label.Document) (Result, error) {
	return renderDocumentWithOptions(doc, documentRenderOptions{threshold: 128})
}

func renderDocumentWithOptions(doc label.Document, opts documentRenderOptions) (Result, error) {
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

	thresholdToMonochromeWithThreshold(canvas, opts.threshold)
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
	case label.ElementQR:
		return drawQRElement(dst, element)
	default:
		return fmt.Errorf("unsupported element type %q", element.Type)
	}
}

func drawQRElement(dst draw.Image, element label.Element) error {
	if element.QR == nil || strings.TrimSpace(element.QR.Value) == "" {
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
	code, err := qrcode.New(strings.TrimSpace(element.QR.Value), qrcode.Medium)
	if err != nil {
		return fmt.Errorf("generate qr: %w", err)
	}
	code.DisableBorder = true
	return drawQR(dst, code, rect)
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
	textRect := rect.Inset(textPaddingPx())
	if textRect.Dx() <= 0 || textRect.Dy() <= 0 {
		return nil
	}

	face, err := newTextFace(element.Text.FontSize, element.Text.FontPath)
	if err != nil {
		return err
	}
	defer face.Close()

	layout := layoutTextWithFace(element.Text.Value, face, textRect.Dx())
	if len(layout.Lines) == 0 {
		return nil
	}

	for i, line := range layout.Lines {
		baselineY := textRect.Min.Y + layout.AscentPx + i*layout.LineHeightPx
		if baselineY > textRect.Max.Y {
			break
		}
		textWidth := font.MeasureString(face, line).Ceil()
		textX := textRect.Min.X + max(0, (textRect.Dx()-textWidth)/2)
		d := font.Drawer{
			Dst:  dst,
			Src:  image.Black,
			Face: face,
			Dot:  fixed.P(textX, baselineY),
		}
		d.DrawString(line)
	}
	return nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
