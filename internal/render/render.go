package render

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"niimcli/internal/api"
	"niimcli/internal/config"
)

const dotsPerMM = 8.0

type Result struct {
	Image        image.Image
	WidthPx      int
	HeightPx     int
	PrintablePx  image.Rectangle
	Shape        string
	Rotation     int
	PreviewPNG   []byte
	PreviewBytes int
}

func QRLabel(req api.PrintRequest, printer config.PrinterProfile, preset config.LabelPreset) (Result, error) {
	if preset.WidthMM <= 0 || preset.HeightMM <= 0 {
		return Result{}, fmt.Errorf("invalid preset dimensions")
	}

	widthPx := mmToPx(preset.WidthMM)
	heightPx := mmToPx(preset.HeightMM)
	widthPx, heightPx = constrainToPrinter(printer, widthPx, heightPx)
	marginPx := mmToPx(preset.MarginsMM)
	if widthPx <= 0 || heightPx <= 0 {
		return Result{}, fmt.Errorf("invalid output size")
	}

	printable := image.Rect(marginPx, marginPx, widthPx-marginPx, heightPx-marginPx)
	if printable.Dx() <= 0 || printable.Dy() <= 0 {
		return Result{}, fmt.Errorf("preset margins leave no printable area")
	}

	canvas := image.NewGray(image.Rect(0, 0, widthPx, heightPx))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	if err := composeLabel(canvas, printable, preset, req); err != nil {
		return Result{}, err
	}

	thresholdToMonochrome(canvas)
	if preset.Shape == "round" {
		maskRound(canvas)
	}

	rotation := normalizedRotation(printer.Defaults.Rotate)
	if rotation != 0 {
		canvas = rotateGray(canvas, rotation)
		printable = rotateRect(printable, widthPx, heightPx, rotation)
		if rotation == 90 || rotation == 270 {
			widthPx, heightPx = heightPx, widthPx
		}
	}

	var preview bytes.Buffer
	if err := png.Encode(&preview, canvas); err != nil {
		return Result{}, fmt.Errorf("encode preview: %w", err)
	}

	return Result{
		Image:        canvas,
		WidthPx:      widthPx,
		HeightPx:     heightPx,
		PrintablePx:  printable,
		Shape:        preset.Shape,
		Rotation:     rotation,
		PreviewPNG:   preview.Bytes(),
		PreviewBytes: preview.Len(),
	}, nil
}

func composeLabel(canvas *image.Gray, printable image.Rectangle, preset config.LabelPreset, req api.PrintRequest) error {
	qrImage, err := qrcode.New(strings.TrimSpace(req.QR.Text), qrcode.Medium)
	if err != nil {
		return fmt.Errorf("generate qr: %w", err)
	}
	qrImage.DisableBorder = true

	layout := normalizedLayout(req.Label.Layout)
	if layout == api.LayoutQROnly {
		return drawQR(canvas, qrImage, fitCentered(image.Rect(0, 0, 1, 1), printable))
	}

	face := basicfont.Face7x13
	spacing := 6
	textLines := make([]string, 0, 2)
	if layout == api.LayoutQRTitle || layout == api.LayoutQRTitleSubtitle {
		textLines = append(textLines, strings.TrimSpace(req.Content.Title))
	}
	if layout == api.LayoutQRTitleSubtitle {
		textLines = append(textLines, strings.TrimSpace(req.Content.Subtitle))
	}

	qrRect, textRect, stacked := chooseLayout(printable, preset, layout, face, spacing)
	if err := drawQR(canvas, qrImage, qrRect); err != nil {
		return err
	}
	if textRect.Dx() <= 0 || textRect.Dy() <= 0 {
		return nil
	}
	drawTextBlock(canvas, textRect, textLines, face, spacing, stacked)
	return nil
}

func chooseLayout(printable image.Rectangle, preset config.LabelPreset, layout api.Layout, face font.Face, spacing int) (image.Rectangle, image.Rectangle, bool) {
	if layout == api.LayoutQROnly {
		return printable, image.Rectangle{}, false
	}

	isRound := strings.EqualFold(preset.Shape, "round")
	if isRound {
		textHeight := face.Metrics().Height.Ceil()
		lineCount := 1
		if layout == api.LayoutQRTitleSubtitle {
			lineCount = 2
		}
		textBlockHeight := lineCount*textHeight + (lineCount-1)*spacing
		gap := 6
		textRect := image.Rect(printable.Min.X, printable.Max.Y-textBlockHeight, printable.Max.X, printable.Max.Y)
		qrRect := image.Rect(printable.Min.X, printable.Min.Y, printable.Max.X, textRect.Min.Y-gap)
		if qrRect.Dy() < mmToPx(12) {
			return printable, image.Rectangle{}, true
		}
		return roundSafeQRRect(qrRect), textRect, true
	}
	if preset.HeightMM > preset.WidthMM {
		return chooseStackedRectLayout(printable, layout, face, spacing)
	}

	gap := 8
	textWidth := printable.Dx() / 2
	if preset.WidthMM >= 70 {
		textWidth = int(float64(printable.Dx()) * 0.4)
	}
	if textWidth < mmToPx(12) {
		textWidth = mmToPx(12)
	}
	textRect := image.Rect(printable.Max.X-textWidth, printable.Min.Y, printable.Max.X, printable.Max.Y)
	qrRect := image.Rect(printable.Min.X, printable.Min.Y, textRect.Min.X-gap, printable.Max.Y)
	if qrRect.Dx() <= 0 || qrRect.Dy() <= 0 {
		return fitCentered(image.Rect(0, 0, 1, 1), printable), image.Rectangle{}, false
	}
	return fitCentered(image.Rect(0, 0, 1, 1), qrRect), textRect, false
}

func chooseStackedRectLayout(printable image.Rectangle, layout api.Layout, face font.Face, spacing int) (image.Rectangle, image.Rectangle, bool) {
	textHeight := face.Metrics().Height.Ceil()
	lineCount := 1
	if layout == api.LayoutQRTitleSubtitle {
		lineCount = 2
	}
	textBlockHeight := lineCount*textHeight + (lineCount-1)*spacing
	gap := 8
	textRect := image.Rect(printable.Min.X, printable.Max.Y-textBlockHeight, printable.Max.X, printable.Max.Y)
	qrRect := image.Rect(printable.Min.X, printable.Min.Y, printable.Max.X, textRect.Min.Y-gap)
	if qrRect.Dx() <= 0 || qrRect.Dy() <= 0 {
		return fitCentered(image.Rect(0, 0, 1, 1), printable), image.Rectangle{}, true
	}
	return fitCentered(image.Rect(0, 0, 1, 1), qrRect), textRect, true
}

func constrainToPrinter(printer config.PrinterProfile, widthPx, heightPx int) (int, int) {
	maxWidth := maxPrintableWidthPx(printer.Model)
	if maxWidth == 0 {
		return widthPx, heightPx
	}
	shortSide := min(widthPx, heightPx)
	if shortSide <= maxWidth {
		return widthPx, heightPx
	}
	scale := float64(maxWidth) / float64(shortSide)
	return int(math.Round(float64(widthPx) * scale)), int(math.Round(float64(heightPx) * scale))
}

func maxPrintableWidthPx(model string) int {
	switch strings.ToUpper(strings.TrimSpace(model)) {
	case "B1", "B18", "B21":
		return 384
	case "D11":
		return 96
	default:
		return 0
	}
}

func roundSafeQRRect(bounds image.Rectangle) image.Rectangle {
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return image.Rectangle{}
	}

	size := min(bounds.Dx(), bounds.Dy())
	if size <= 0 {
		return image.Rectangle{}
	}

	// Keep the QR inside the inscribed circle rather than the full square bounds.
	safeSize := int(math.Floor(float64(size) / math.Sqrt2))
	padding := max(4, size/20)
	safeSize -= padding * 2
	if safeSize < 1 {
		safeSize = 1
	}

	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/2
	half := safeSize / 2
	minX := centerX - half
	minY := centerY - half
	return image.Rect(minX, minY, minX+safeSize, minY+safeSize)
}

func drawQR(dst draw.Image, code *qrcode.QRCode, rect image.Rectangle) error {
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return nil
	}
	size := min(rect.Dx(), rect.Dy())
	if size <= 0 {
		return nil
	}
	pngData, err := code.PNG(size)
	if err != nil {
		return fmt.Errorf("encode qr: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return fmt.Errorf("decode qr: %w", err)
	}
	centered := fitCentered(img.Bounds(), rect)
	scaleNearest(dst, centered, img, img.Bounds())
	return nil
}

func drawTextBlock(dst draw.Image, rect image.Rectangle, lines []string, face font.Face, spacing int, centered bool) {
	if len(lines) == 0 {
		return
	}
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	blockHeight := len(lines)*lineHeight + (len(lines)-1)*spacing
	baseline := rect.Min.Y + metrics.Ascent.Ceil()
	if centered {
		baseline = rect.Min.Y + max(0, (rect.Dy()-blockHeight)/2) + metrics.Ascent.Ceil()
	}

	for i, line := range lines {
		trimmed := fitText(line, face, rect.Dx())
		if trimmed == "" {
			continue
		}
		y := baseline + i*(lineHeight+spacing)
		if y > rect.Max.Y {
			break
		}
		x := rect.Min.X
		if centered {
			lineWidth := font.MeasureString(face, trimmed).Ceil()
			x = rect.Min.X + max(0, (rect.Dx()-lineWidth)/2)
		}
		d := font.Drawer{
			Dst:  dst,
			Src:  image.Black,
			Face: face,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(trimmed)
	}
}

func fitText(s string, face font.Face, maxWidth int) string {
	s = strings.TrimSpace(s)
	if s == "" || maxWidth <= 0 {
		return ""
	}
	if font.MeasureString(face, s).Ceil() <= maxWidth {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimSpace(string(runes)) + "..."
		if font.MeasureString(face, candidate).Ceil() <= maxWidth {
			return candidate
		}
	}
	return ""
}

func mmToPx(mm float64) int {
	return int(math.Round(mm * dotsPerMM))
}

func fitCentered(src image.Rectangle, dst image.Rectangle) image.Rectangle {
	srcW := src.Dx()
	srcH := src.Dy()
	if srcW <= 0 || srcH <= 0 {
		return image.Rect(dst.Min.X, dst.Min.Y, dst.Min.X, dst.Min.Y)
	}

	scale := math.Min(float64(dst.Dx())/float64(srcW), float64(dst.Dy())/float64(srcH))
	if scale <= 0 {
		return image.Rect(dst.Min.X, dst.Min.Y, dst.Min.X, dst.Min.Y)
	}

	outW := int(math.Round(float64(srcW) * scale))
	outH := int(math.Round(float64(srcH) * scale))
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}

	offsetX := dst.Min.X + (dst.Dx()-outW)/2
	offsetY := dst.Min.Y + (dst.Dy()-outH)/2
	return image.Rect(offsetX, offsetY, offsetX+outW, offsetY+outH)
}

func scaleNearest(dst draw.Image, dstRect image.Rectangle, src image.Image, srcRect image.Rectangle) {
	if dstRect.Dx() <= 0 || dstRect.Dy() <= 0 || srcRect.Dx() <= 0 || srcRect.Dy() <= 0 {
		return
	}

	for y := dstRect.Min.Y; y < dstRect.Max.Y; y++ {
		sy := srcRect.Min.Y + ((y-dstRect.Min.Y)*srcRect.Dy())/dstRect.Dy()
		for x := dstRect.Min.X; x < dstRect.Max.X; x++ {
			sx := srcRect.Min.X + ((x-dstRect.Min.X)*srcRect.Dx())/dstRect.Dx()
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func thresholdToMonochrome(img *image.Gray) {
	thresholdToMonochromeWithThreshold(img, 128)
}

func thresholdToMonochromeWithThreshold(img *image.Gray, threshold uint8) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.GrayAt(x, y).Y < threshold {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
}

func maskRound(img *image.Gray) {
	b := img.Bounds()
	cx := float64(b.Min.X+b.Max.X-1) / 2
	cy := float64(b.Min.Y+b.Max.Y-1) / 2
	radius := math.Min(float64(b.Dx()), float64(b.Dy())) / 2
	radiusSq := radius * radius

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			if dx*dx+dy*dy > radiusSq {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
}

func normalizedRotation(rotate int) int {
	r := rotate % 360
	if r < 0 {
		r += 360
	}
	switch r {
	case 0, 90, 180, 270:
		return r
	default:
		return 0
	}
}

func rotateGray(src *image.Gray, rotation int) *image.Gray {
	rotation = normalizedRotation(rotation)
	if rotation == 0 {
		return src
	}

	sb := src.Bounds()
	var dst *image.Gray
	if rotation == 90 || rotation == 270 {
		dst = image.NewGray(image.Rect(0, 0, sb.Dy(), sb.Dx()))
	} else {
		dst = image.NewGray(image.Rect(0, 0, sb.Dx(), sb.Dy()))
	}

	for y := sb.Min.Y; y < sb.Max.Y; y++ {
		for x := sb.Min.X; x < sb.Max.X; x++ {
			v := src.GrayAt(x, y)
			sx := x - sb.Min.X
			sy := y - sb.Min.Y
			switch rotation {
			case 90:
				dst.SetGray(sb.Dy()-1-sy, sx, v)
			case 180:
				dst.SetGray(sb.Dx()-1-sx, sb.Dy()-1-sy, v)
			case 270:
				dst.SetGray(sy, sb.Dx()-1-sx, v)
			}
		}
	}

	return dst
}

func rotateRect(r image.Rectangle, width, height, rotation int) image.Rectangle {
	rotation = normalizedRotation(rotation)
	if rotation == 0 {
		return r
	}

	points := []image.Point{
		{X: r.Min.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Min.Y},
		{X: r.Min.X, Y: r.Max.Y},
		{X: r.Max.X, Y: r.Max.Y},
	}
	rotated := make([]image.Point, 0, len(points))
	for _, p := range points {
		switch rotation {
		case 90:
			rotated = append(rotated, image.Point{X: height - p.Y, Y: p.X})
		case 180:
			rotated = append(rotated, image.Point{X: width - p.X, Y: height - p.Y})
		case 270:
			rotated = append(rotated, image.Point{X: p.Y, Y: width - p.X})
		}
	}

	minX, minY := rotated[0].X, rotated[0].Y
	maxX, maxY := rotated[0].X, rotated[0].Y
	for _, p := range rotated[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	if minX < 0 || minY < 0 {
		shiftX, shiftY := 0, 0
		if minX < 0 {
			shiftX = -minX
		}
		if minY < 0 {
			shiftY = -minY
		}
		minX += shiftX
		maxX += shiftX
		minY += shiftY
		maxY += shiftY
	}

	return image.Rect(minX, minY, maxX, maxY)
}

func normalizedLayout(layout api.Layout) api.Layout {
	if layout == "" {
		return api.LayoutQROnly
	}
	return layout
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
