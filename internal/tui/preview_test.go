package tui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

func TestPrintCurrentDocumentKeepsSessionOpen(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{Session: noopPrinterSession{}, Copies: 1})
	m.Connection = ConnectionConnected
	cmd := m.printCurrentDocument()
	if cmd == nil {
		t.Fatal("printCurrentDocument() did not start a print")
	}
	msg, ok := cmd().(printResultMsg)
	if !ok || !msg.OK || msg.Err != nil {
		t.Fatalf("printCurrentDocument() = %#v, want successful print", msg)
	}
	cmd = m.handlePrintResult(msg)
	if cmd != nil || m.Connection != ConnectionConnected {
		t.Fatalf("print result scheduled reconnect: command %v, connection %s", cmd, m.Connection)
	}
}

func TestPrintRejectsMismatchedInstalledRoll(t *testing.T) {
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{Session: noopPrinterSession{}, Model: "B1", Copies: 1}, []config.LabelPreset{{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect"}}, "b1-50x30")
	m.Connection = ConnectionConnected
	if cmd := m.printCurrentDocument(); cmd != nil || !strings.Contains(m.Status, "does not match") {
		t.Fatalf("print command = %v, status = %q", cmd, m.Status)
	}
}

func TestRotatingB150x80CanvasOrientsLivePreviewWithoutChangingPrintSize(t *testing.T) {
	session := &capturingPrinterSession{}
	stock := config.LabelPreset{Name: "b1-50x80", WidthMM: 50, HeightMM: 80, Shape: "rect"}
	m := NewModelWithPresets(50, 80, "rect", "", PrintConfig{Session: session, Model: "B1", Copies: 1}, []config.LabelPreset{stock}, stock.Name)
	m.Connection = ConnectionConnected
	m.Width, m.Height = 180, 36
	m.reflow()
	m.Document.Elements = []label.Element{{ID: "qr", Type: label.ElementQR, XMM: 0, YMM: 0, WidthMM: 50, HeightMM: 50, QR: &label.QRElement{Value: "https://example.test"}}}
	before := renderLivePreviewCmd(m, 1)().(livePreviewRenderedMsg)
	if before.WidthPx != 384 || before.HeightPx != 640 {
		t.Fatalf("original preview = %dx%d, want 384x640", before.WidthPx, before.HeightPx)
	}
	beforeQR := darkBounds(t, before.PNG)

	for rotation := 90; rotation <= 360; rotation += 90 {
		if !m.rotateCanvas() {
			t.Fatal("rotateCanvas() failed")
		}
		width, height := m.Document.PhysicalDimensionsMM()
		if width != 50 || height != 80 {
			t.Fatalf("%d-degree label dimensions = %.1fx%.1fmm, want 50x80mm", rotation, width, height)
		}
		after := renderLivePreviewCmd(m, rotation)().(livePreviewRenderedMsg)
		wantWidth, wantHeight := before.WidthPx, before.HeightPx
		if rotation%180 != 0 {
			wantWidth, wantHeight = wantHeight, wantWidth
		}
		if after.WidthPx != wantWidth || after.HeightPx != wantHeight {
			t.Fatalf("%d-degree preview = %dx%d, want %dx%d", rotation, after.WidthPx, after.HeightPx, wantWidth, wantHeight)
		}
		qr := darkBounds(t, after.PNG)
		wantQRWidth, wantQRHeight := beforeQR.Dx(), beforeQR.Dy()
		if rotation%180 != 0 {
			wantQRWidth, wantQRHeight = wantQRHeight, wantQRWidth
		}
		if math.Abs(float64(qr.Dx()-wantQRWidth)) > 2 || math.Abs(float64(qr.Dy()-wantQRHeight)) > 2 {
			t.Fatalf("%d-degree QR size = %dx%d, want %dx%d", rotation, qr.Dx(), qr.Dy(), wantQRWidth, wantQRHeight)
		}
		if m.Preset < 0 {
			t.Fatalf("%d-degree rotation lost the installed 50x80 roll", rotation)
		}
		cmd := m.printCurrentDocument()
		if cmd == nil {
			t.Fatalf("%d-degree document could not print: %s", rotation, m.Status)
		}
		if msg := cmd().(printResultMsg); !msg.OK {
			t.Fatalf("%d-degree print failed: %#v", rotation, msg)
		}
		if session.result.WidthPx != 400 || session.result.HeightPx != 640 {
			t.Fatalf("%d-degree print raster = %dx%d, want 400x640 before B1 fitting", rotation, session.result.WidthPx, session.result.HeightPx)
		}
	}
}

func TestRotatingCanvasKeepsTextSizeInLivePreview(t *testing.T) {
	m := NewModel(50, 80, "rect", "", PrintConfig{Model: "B1"})
	m.Width, m.Height = 180, 36
	m.reflow()
	m.Document.Elements = []label.Element{{
		ID: "title", Type: label.ElementText, XMM: 5, YMM: 5, WidthMM: 40, HeightMM: 12,
		Text: &label.TextElement{Value: "Shelf label", FontSize: 24},
	}}
	before := renderLivePreviewCmd(m, 1)().(livePreviewRenderedMsg)
	beforeText := darkBounds(t, before.PNG)
	if !m.rotateCanvas() {
		t.Fatal("rotateCanvas() failed")
	}
	after := renderLivePreviewCmd(m, 2)().(livePreviewRenderedMsg)
	afterText := darkBounds(t, after.PNG)
	if math.Abs(float64(afterText.Dx()-beforeText.Dy())) > 2 || math.Abs(float64(afterText.Dy()-beforeText.Dx())) > 2 {
		t.Fatalf("rotated text size = %dx%d, want %dx%d", afterText.Dx(), afterText.Dy(), beforeText.Dy(), beforeText.Dx())
	}
}

func darkBounds(t *testing.T, data []byte) image.Rectangle {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	bounds := img.Bounds()
	dark := image.Rectangle{Min: bounds.Max, Max: bounds.Min}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128 {
				dark = dark.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if dark.Empty() {
		t.Fatal("preview has no content")
	}
	return dark
}

type capturingPrinterSession struct {
	noopPrinterSession
	result render.Result
}

func (s *capturingPrinterSession) PrintImage(_ context.Context, result render.Result, _ int) api.PrintResponse {
	s.result = result
	return api.PrintResponse{OK: true}
}
