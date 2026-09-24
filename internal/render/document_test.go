package render

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"testing"

	"github.com/skip2/go-qrcode"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/label"
)

func TestRenderDocumentProducesPNG(t *testing.T) {
	doc := label.NewDocument(50, 30)
	if err := doc.AddElement(label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	if result.WidthPx != 400 || result.HeightPx != 240 {
		t.Fatalf("render size = %dx%d, want 400x240", result.WidthPx, result.HeightPx)
	}
	if len(result.PreviewPNG) == 0 {
		t.Fatal("RenderDocument() returned empty preview")
	}
	img, err := png.Decode(bytes.NewReader(result.PreviewPNG))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, 400, 240) {
		t.Fatalf("preview bounds = %v, want %v", img.Bounds(), image.Rect(0, 0, 400, 240))
	}
}

func TestRenderDocumentDrawsText(t *testing.T) {
	doc := label.NewDocument(40, 20)
	if err := doc.AddElement(label.NewTextElement("title", "Test", 2, 2, 20, 8, 18)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}

	blackPixels := 0
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			if gray.GrayAt(x, y).Y == 0 {
				blackPixels++
			}
		}
	}
	if blackPixels == 0 {
		t.Fatal("expected rendered text to produce black pixels")
	}
}

func TestRenderDocumentDrawsQR(t *testing.T) {
	doc := label.NewDocument(40, 40)
	if err := doc.AddElement(label.NewQRElement("qr", "https://example.com", 8, 8, 24)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}

	blackPixels := 0
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			if gray.GrayAt(x, y).Y == 0 {
				blackPixels++
			}
		}
	}
	if blackPixels == 0 {
		t.Fatal("expected rendered QR to produce black pixels")
	}
}

func TestRenderDocumentDrawsRotatedElements(t *testing.T) {
	doc := label.NewDocument(50, 30)
	text := label.NewTextElement("title", "Rotated", 4, 4, 20, 8, 18)
	text.Rotation = 90
	qr := label.NewQRElement("qr", "https://example.com", 30, 6, 16)
	qr.Rotation = 90
	if err := doc.AddElement(text); err != nil {
		t.Fatalf("AddElement(text) error = %v", err)
	}
	if err := doc.AddElement(qr); err != nil {
		t.Fatalf("AddElement(qr) error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}
	blackPixels := 0
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			if gray.GrayAt(x, y).Y == 0 {
				blackPixels++
			}
		}
	}
	if blackPixels == 0 {
		t.Fatal("expected rotated elements to produce black pixels")
	}
}

func TestRenderDocumentLeavesQRElementInset(t *testing.T) {
	element := label.NewQRElement("qr", "https://example.com", 8, 8, 24)
	doc := label.NewDocument(40, 40)
	if err := doc.AddElement(element); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}

	rect := image.Rect(mmToPx(element.XMM), mmToPx(element.YMM), mmToPx(element.XMM+element.WidthMM), mmToPx(element.YMM+element.HeightMM))
	inset := qrElementInsetPx(rect)
	if inset <= 0 {
		t.Fatalf("qrElementInsetPx() = %d, want positive", inset)
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			insideInset := x >= rect.Min.X+inset && x < rect.Max.X-inset && y >= rect.Min.Y+inset && y < rect.Max.Y-inset
			if insideInset {
				continue
			}
			if gray.GrayAt(x, y).Y == 0 {
				t.Fatalf("found black QR pixel in inset at (%d,%d)", x, y)
			}
		}
	}
}

func TestDrawQRUsesIntegerModuleScale(t *testing.T) {
	code, err := qrcode.New("https://example.com", qrcode.Medium)
	if err != nil {
		t.Fatalf("qrcode.New() error = %v", err)
	}
	code.DisableBorder = true
	bitmap := code.Bitmap()
	moduleCount := len(bitmap)
	canvas := image.NewGray(image.Rect(0, 0, moduleCount*3+2, moduleCount*3+2))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)

	if err := drawQR(canvas, code, canvas.Bounds()); err != nil {
		t.Fatalf("drawQR() error = %v", err)
	}

	left := (canvas.Bounds().Dx() - moduleCount*3) / 2
	top := (canvas.Bounds().Dy() - moduleCount*3) / 2
	for y, row := range bitmap {
		for x, on := range row {
			want := uint8(255)
			if on {
				want = 0
			}
			for py := top + y*3; py < top+(y+1)*3; py++ {
				for px := left + x*3; px < left+(x+1)*3; px++ {
					if got := canvas.GrayAt(px, py).Y; got != want {
						t.Fatalf("pixel (%d,%d) = %d, want %d", px, py, got, want)
					}
				}
			}
		}
	}
}

func TestCalibrationLabelDrawsAxisAlignedPattern(t *testing.T) {
	result, err := CalibrationLabel(50, 30, "rect")
	if err != nil {
		t.Fatalf("CalibrationLabel() error = %v", err)
	}
	if result.WidthPx != 400 || result.HeightPx != 240 {
		t.Fatalf("calibration size = %dx%d, want 400x240", result.WidthPx, result.HeightPx)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}
	if gray.GrayAt(24, 24).Y != 0 {
		t.Fatalf("top-left border pixel = %d, want black", gray.GrayAt(24, 24).Y)
	}
	if gray.GrayAt(result.WidthPx/2, result.HeightPx/2).Y != 0 {
		t.Fatalf("center cross pixel = %d, want black", gray.GrayAt(result.WidthPx/2, result.HeightPx/2).Y)
	}
}

func TestQRLabelConstrainsB1Width(t *testing.T) {
	result, err := QRLabel(api.PrintRequest{
		QR: api.QRRequest{Text: "https://example.com"},
	}, config.PrinterProfile{Model: "B1"}, config.LabelPreset{WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: string(api.LayoutQROnly)})
	if err != nil {
		t.Fatalf("QRLabel() error = %v", err)
	}
	if result.WidthPx != 384 {
		t.Fatalf("width = %d, want 384", result.WidthPx)
	}
	if result.HeightPx != 240 {
		t.Fatalf("height = %d, want 240", result.HeightPx)
	}
}

func TestLayoutTextWrapsWordsIntoMultipleLines(t *testing.T) {
	layout, err := LayoutText("Storage Box", 18, mmToPx(12))
	if err != nil {
		t.Fatalf("LayoutText() error = %v", err)
	}
	if len(layout.Lines) < 2 {
		t.Fatalf("line count = %d, want at least 2", len(layout.Lines))
	}
	if layout.BlockHeightPx <= layout.LineHeightPx {
		t.Fatalf("block height = %d, want greater than line height %d", layout.BlockHeightPx, layout.LineHeightPx)
	}
}

func TestRequiredTextHeightIncreasesWhenWidthShrinks(t *testing.T) {
	element := label.NewTextElement("title", "Storage Box 12", 0, 0, 20, 6, 18)
	wideHeight, err := RequiredTextHeightMM(element, 20)
	if err != nil {
		t.Fatalf("RequiredTextHeightMM() wide error = %v", err)
	}
	narrowHeight, err := RequiredTextHeightMM(element, 10)
	if err != nil {
		t.Fatalf("RequiredTextHeightMM() narrow error = %v", err)
	}
	if narrowHeight <= wideHeight {
		t.Fatalf("narrow height = %.2f, want greater than wide height %.2f", narrowHeight, wideHeight)
	}
}

func TestRequiredTextHeightUsesTightTextPadding(t *testing.T) {
	element := label.NewTextElement("title", "Box", 0, 0, 30, 8, 18)
	height, err := RequiredTextHeightMM(element, 30)
	if err != nil {
		t.Fatalf("RequiredTextHeightMM() error = %v", err)
	}
	layout, err := LayoutTextWithFontPath(element.Text.Value, element.Text.FontSize, element.Text.FontPath, mmToPx(30)-textPaddingPx()*2)
	if err != nil {
		t.Fatalf("LayoutTextWithFontPath() error = %v", err)
	}
	safetyPx := textInkTopSafetyPx + textInkBottomSafetyPx
	if layout.BlockHeightPx > layout.LineHeightPx+safetyPx {
		t.Fatalf("block height = %d, want no greater than line height %d plus safety %d", layout.BlockHeightPx, layout.LineHeightPx, safetyPx)
	}
	want := pxToMM(layout.BlockHeightPx + 2*textPaddingPx())
	if height != want {
		t.Fatalf("height = %.2f, want %.2f", height, want)
	}
	if textPaddingPx() != 4 {
		t.Fatalf("textPaddingPx() = %d, want 4 for 0.5mm at 8 dots/mm", textPaddingPx())
	}
}

func TestLayoutTextKeepsInkSafety(t *testing.T) {
	layout, err := LayoutText("Box", 18, mmToPx(30))
	if err != nil {
		t.Fatalf("LayoutText() error = %v", err)
	}
	if layout.AscentPx <= 0 {
		t.Fatalf("ascent = %d, want positive", layout.AscentPx)
	}
	if layout.BlockHeightPx <= textInkTopSafetyPx+textInkBottomSafetyPx {
		t.Fatalf("block height = %d, want greater than safety %d", layout.BlockHeightPx, textInkTopSafetyPx+textInkBottomSafetyPx)
	}
}

func TestRenderDocumentLeavesTextTopPadding(t *testing.T) {
	element := label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)
	doc := label.NewDocument(50, 30)
	if err := doc.AddElement(element); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}

	left := mmToPx(element.XMM)
	right := mmToPx(element.XMM + element.WidthMM)
	top := mmToPx(element.YMM)
	paddingBottom := top + textPaddingPx()
	for y := top; y < paddingBottom; y++ {
		for x := left; x < right; x++ {
			if gray.GrayAt(x, y).Y == 0 {
				t.Fatalf("found black pixel in top padding at (%d,%d)", x, y)
			}
		}
	}
}

func TestRenderDocumentOutputIsMonochrome(t *testing.T) {
	doc := label.NewDocument(50, 30)
	if err := doc.AddElement(label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	result, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	gray, ok := result.Image.(*image.Gray)
	if !ok {
		t.Fatalf("render image type = %T, want *image.Gray", result.Image)
	}
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			v := gray.GrayAt(x, y).Y
			if v != 0 && v != 255 {
				t.Fatalf("pixel at (%d,%d) = %d, want monochrome", x, y, v)
			}
		}
	}
}

func TestRenderDocumentForTUIUsesSameCanvasSize(t *testing.T) {
	doc := label.NewDocument(50, 30)
	if err := doc.AddElement(label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	printResult, err := RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v", err)
	}
	tuiResult, err := RenderDocumentForTUI(doc)
	if err != nil {
		t.Fatalf("RenderDocumentForTUI() error = %v", err)
	}
	if tuiResult.WidthPx != printResult.WidthPx || tuiResult.HeightPx != printResult.HeightPx {
		t.Fatalf("TUI render size = %dx%d, want %dx%d", tuiResult.WidthPx, tuiResult.HeightPx, printResult.WidthPx, printResult.HeightPx)
	}
	if len(tuiResult.PreviewPNG) == 0 {
		t.Fatal("RenderDocumentForTUI() returned empty preview")
	}
}

func TestRenderDocumentRejectsInvalidFontPath(t *testing.T) {
	doc := label.NewDocument(50, 30)
	element := label.NewTextElement("title", "Storage Box 12", 5, 4, 30, 8, 18)
	element.Text.FontPath = "/no/such/font.ttf"
	if err := doc.AddElement(element); err != nil {
		t.Fatalf("AddElement() error = %v", err)
	}

	if _, err := RenderDocument(doc); err == nil {
		t.Fatal("RenderDocument() error = nil, want invalid font path error")
	}
}

func TestValidateFontPathAllowsFallback(t *testing.T) {
	if err := ValidateFontPath(""); err != nil {
		t.Fatalf("ValidateFontPath() fallback error = %v", err)
	}
}
