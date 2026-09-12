package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

	"niimtui/internal/api"
	"niimtui/internal/config"
)

func TestValidateRequestUsesDefaultPresetAndQROnlyDefaultLayout(t *testing.T) {
	svc := mustService(t)
	req := api.PrintRequest{
		Printer: api.PrinterSelector{Selector: "d110-desk"},
		QR:      api.QRRequest{Text: "https://example.test/item/1"},
	}

	printer, preset, errResp := svc.validateRequest(req)
	if errResp != nil {
		t.Fatalf("validateRequest() unexpected error = %#v", errResp)
	}
	if printer.Name != "d110-desk" {
		t.Fatalf("printer = %q, want d110-desk", printer.Name)
	}
	if preset.Name != "d110-12x40" {
		t.Fatalf("preset = %q, want d110-12x40", preset.Name)
	}
	if got := normalizedLayout(req.Label.Layout); got != api.LayoutQROnly {
		t.Fatalf("normalizedLayout() = %q, want %q", got, api.LayoutQROnly)
	}
}

func TestValidateRequestRejectsMissingQRText(t *testing.T) {
	svc := mustService(t)
	req := api.PrintRequest{Printer: api.PrinterSelector{Selector: "d110-desk"}}

	_, _, errResp := svc.validateRequest(req)
	if errResp == nil || errResp.Error == nil {
		t.Fatalf("validateRequest() expected error")
	}
	if errResp.Error.Code != ErrInvalidRequest {
		t.Fatalf("error code = %q, want %q", errResp.Error.Code, ErrInvalidRequest)
	}
}

func TestValidateRequestRejectsUnsupportedLayout(t *testing.T) {
	svc := mustService(t)
	req := api.PrintRequest{
		Printer: api.PrinterSelector{Selector: "d110-desk"},
		Label:   api.LabelRequest{Layout: api.Layout("full-image")},
		QR:      api.QRRequest{Text: "https://example.test/item/1"},
	}

	_, _, errResp := svc.validateRequest(req)
	if errResp == nil || errResp.Error == nil {
		t.Fatalf("validateRequest() expected error")
	}
	if errResp.Error.Code != ErrInvalidRequest {
		t.Fatalf("error code = %q, want %q", errResp.Error.Code, ErrInvalidRequest)
	}
}

func TestValidateRequestRejectsUnknownPrinter(t *testing.T) {
	svc := mustService(t)
	req := api.PrintRequest{
		Printer: api.PrinterSelector{Selector: "missing"},
		QR:      api.QRRequest{Text: "https://example.test/item/1"},
	}

	_, _, errResp := svc.validateRequest(req)
	if errResp == nil || errResp.Error == nil {
		t.Fatalf("validateRequest() expected error")
	}
	if errResp.Error.Code != ErrPrinterNotFound {
		t.Fatalf("error code = %q, want %q", errResp.Error.Code, ErrPrinterNotFound)
	}
}

func TestRenderPreviewProducesPNG(t *testing.T) {
	svc := mustService(t)
	requests := []api.PrintRequest{
		{
			Printer: api.PrinterSelector{Selector: "b1-50x30"},
			Label:   api.LabelRequest{Preset: "b1-50x30", Layout: api.LayoutQRTitle},
			QR:      api.QRRequest{Text: "https://example.test/item/50x30"},
			Content: api.ContentRequest{Title: "Bin 30"},
		},
		{
			Printer: api.PrinterSelector{Selector: "b1-round"},
			Label:   api.LabelRequest{Preset: "b1-50x50-round", Layout: api.LayoutQRTitleSubtitle},
			QR:      api.QRRequest{Text: "https://example.test/item/50x50"},
			Content: api.ContentRequest{Title: "Decor", Subtitle: "Shelf 2"},
		},
		{
			Printer: api.PrinterSelector{Selector: "b1-50x80"},
			Label:   api.LabelRequest{Preset: "b1-50x80", Layout: api.LayoutQRTitleSubtitle},
			QR:      api.QRRequest{Text: "https://example.test/item/50x80"},
			Content: api.ContentRequest{Title: "Hardware", Subtitle: "Wall Rack"},
		},
	}

	for _, req := range requests {
		preview, err := svc.RenderPreview(context.Background(), req)
		if err != nil {
			t.Fatalf("RenderPreview() error for %q = %v", req.Label.Preset, err)
		}
		if len(preview) == 0 {
			t.Fatalf("RenderPreview() returned empty preview for %q", req.Label.Preset)
		}
		if _, err := png.Decode(bytes.NewReader(preview)); err != nil {
			t.Fatalf("preview for %q is not a valid PNG: %v", req.Label.Preset, err)
		}
	}
}

func TestRoundPreviewUsesExpectedCanvasSize(t *testing.T) {
	svc := mustService(t)
	req := api.PrintRequest{
		Printer: api.PrinterSelector{Selector: "b1-round"},
		Label:   api.LabelRequest{Preset: "b1-50x50-round", Layout: api.LayoutQRTitleSubtitle},
		QR:      api.QRRequest{Text: "https://example.test/item/round"},
		Content: api.ContentRequest{Title: "Decor", Subtitle: "Shelf 2"},
	}

	preview, err := svc.RenderPreview(context.Background(), req)
	if err != nil {
		t.Fatalf("RenderPreview() error = %v", err)
	}
	img, err := png.Decode(bytes.NewReader(preview))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, 384, 384) {
		t.Fatalf("preview bounds = %v, want %v", img.Bounds(), image.Rect(0, 0, 384, 384))
	}
}

func TestResolvePrinterIncludesIdentifierProfiles(t *testing.T) {
	svc := mustService(t)
	printer, errResp := svc.resolvePrinter("b1-round")
	if errResp != nil {
		t.Fatalf("resolvePrinter() unexpected error = %#v", errResp)
	}
	if printer.Identifier == "" {
		t.Fatal("resolvePrinter() returned printer without identifier")
	}
}

func TestResolvePrinterUsesOnlyConfiguredPrinterByDefault(t *testing.T) {
	svc, err := New(config.Config{
		Printers: []config.PrinterProfile{{Name: "b1-default", Model: "B1", Transport: "ble", DeviceName: "B1-Test", DefaultPreset: "b1-50x50-round"}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	printer, errResp := svc.resolvePrinter("")
	if errResp != nil {
		t.Fatalf("resolvePrinter() unexpected error = %#v", errResp)
	}
	if printer.Name != "b1-default" {
		t.Fatalf("printer = %q, want b1-default", printer.Name)
	}
}

func mustService(t *testing.T) *Service {
	t.Helper()
	cfg := config.Config{
		Server: config.ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test-token", AllowedOrigins: []string{"https://homebox.example"}},
		Printers: []config.PrinterProfile{
			{
				Name:          "d110-desk",
				Model:         "D110",
				Transport:     "ble",
				DeviceName:    "D110_M-H913040249",
				Identifier:    "ea88cc93-a2a2-8287-1f08-18cd0a81649b",
				DefaultPreset: "d110-12x40",
				Defaults:      config.PrinterDefaults{Density: 3},
			},
			{
				Name:          "b1-50x30",
				Model:         "B1",
				Transport:     "ble",
				DeviceName:    "B1-I427031488",
				Identifier:    "e6bc3bef-5a60-3bc7-ffab-50edc0e9f122",
				DefaultPreset: "b1-50x30",
				Defaults:      config.PrinterDefaults{Density: 3},
			},
			{
				Name:          "b1-round",
				Model:         "B1",
				Transport:     "ble",
				DeviceName:    "B1-I427031488",
				Identifier:    "e6bc3bef-5a60-3bc7-ffab-50edc0e9f122",
				DefaultPreset: "b1-50x50-round",
				Defaults:      config.PrinterDefaults{Density: 3},
			},
			{
				Name:          "b1-50x80",
				Model:         "B1",
				Transport:     "ble",
				DeviceName:    "B1-I427031488",
				Identifier:    "e6bc3bef-5a60-3bc7-ffab-50edc0e9f122",
				DefaultPreset: "b1-50x80",
				Defaults:      config.PrinterDefaults{Density: 3},
			},
		},
		Presets: []config.LabelPreset{
			{Name: "d110-12x40", WidthMM: 40, HeightMM: 12, Shape: "rect", Layout: "qr-only", MarginsMM: 1},
			{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-title", MarginsMM: 1},
			{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-title-subtitle", MarginsMM: 2},
			{Name: "b1-50x80", WidthMM: 50, HeightMM: 80, Shape: "rect", Layout: "qr-title-subtitle", MarginsMM: 2},
		},
	}
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return svc
}
