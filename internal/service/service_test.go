package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"sync"
	"testing"
	"time"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/render"
	"niimtui/internal/transport"
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
	if img.Bounds() != image.Rect(0, 0, 384, 400) {
		t.Fatalf("preview bounds = %v, want %v", img.Bounds(), image.Rect(0, 0, 384, 400))
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

func TestResolvePrinterUsesActivePrinterByDefault(t *testing.T) {
	svc := mustService(t)
	svc.cfg.ActivePrinter = "b1-round"
	printer, errResp := svc.resolvePrinter("")
	if errResp != nil {
		t.Fatalf("resolvePrinter() unexpected error = %#v", errResp)
	}
	if printer.Name != "b1-round" {
		t.Fatalf("printer = %q, want b1-round", printer.Name)
	}
}

func TestServiceReusesPrinterConnectionAndClosesOnShutdown(t *testing.T) {
	svc := mustService(t)
	backend := &testBackend{}
	svc.transport = transport.NewManager(backend)

	for _, selector := range []string{"b1-round", "b1-50x80"} {
		resp := svc.Print(context.Background(), api.PrintRequest{
			Printer: api.PrinterSelector{Selector: selector},
			QR:      api.QRRequest{Text: "https://example.test/item"},
		})
		if !resp.OK {
			t.Fatalf("Print(%q) = %#v", selector, resp.Error)
		}
	}
	if backend.connections() != 1 {
		t.Fatalf("connections = %d, want one for two profiles of the same device", backend.connections())
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if backend.closed() != 1 {
		t.Fatalf("closed connections = %d, want 1", backend.closed())
	}
}

func TestProbeSharesConnectionWithPrinting(t *testing.T) {
	svc := mustService(t)
	backend := &testBackend{}
	svc.transport = transport.NewManager(backend)
	t.Cleanup(func() { _ = svc.Close() })
	req := api.PrintRequest{Printer: api.PrinterSelector{Selector: "b1-round"}, QR: api.QRRequest{Text: "https://example.test/item"}}
	if resp := svc.Print(context.Background(), req); !resp.OK {
		t.Fatalf("Print() = %#v", resp)
	}
	if _, errResp := svc.Probe(context.Background(), "b1-round"); errResp != nil {
		t.Fatalf("Probe() = %#v", errResp)
	}
	if backend.connections() != 1 || backend.probes() != 1 {
		t.Fatalf("connections = %d, probes = %d; want 1 and 1", backend.connections(), backend.probes())
	}
}

func TestServiceDropsFailedConnectionAndReconnectsOnNextPrint(t *testing.T) {
	svc := mustService(t)
	backend := &testBackend{failNextPrint: true}
	svc.transport = transport.NewManager(backend)
	t.Cleanup(func() { _ = svc.Close() })
	req := api.PrintRequest{Printer: api.PrinterSelector{Selector: "b1-round"}, QR: api.QRRequest{Text: "https://example.test/item"}}
	if resp := svc.Print(context.Background(), req); resp.OK || resp.Error.Code != ErrPrintFailed {
		t.Fatalf("first Print() = %#v, want print failure", resp)
	}
	if resp := svc.Print(context.Background(), req); !resp.OK {
		t.Fatalf("second Print() = %#v, want recovery", resp)
	}
	if backend.connections() != 2 || backend.closed() != 1 {
		t.Fatalf("connections = %d, closed = %d; want 2 and 1", backend.connections(), backend.closed())
	}
}

func TestSessionConnectPreparesAndKeepsSuccessfulConnection(t *testing.T) {
	svc := mustService(t)
	backend := &testBackend{}
	svc.transport = transport.NewManager(backend)
	session, err := svc.NewSession("b1-round")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if _, err := session.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	if _, err := session.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect() = %v", err)
	}
	if got := backend.prepared(); got != 1 {
		t.Fatalf("prepare calls = %d, want 1", got)
	}
	rendered := render.Result{Image: image.NewGray(image.Rect(0, 0, 8, 8)), WidthPx: 8, HeightPx: 8, PrintablePx: image.Rect(0, 0, 8, 8)}
	for range 2 {
		if resp := session.PrintImage(context.Background(), rendered, 1); !resp.OK {
			t.Fatalf("PrintImage() = %#v", resp)
		}
	}
	if backend.connections() != 1 || backend.closed() != 0 {
		t.Fatalf("connections = %d, closed = %d before session exit; want 1 and 0", backend.connections(), backend.closed())
	}
}

func TestSessionClosesConnectionAfterFailedRetry(t *testing.T) {
	svc := mustService(t)
	backend := &testBackend{failEveryPrint: true}
	svc.transport = transport.NewManager(backend)
	session, err := svc.NewSession("b1-round")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	rendered := render.Result{Image: image.NewGray(image.Rect(0, 0, 8, 8)), WidthPx: 8, HeightPx: 8, PrintablePx: image.Rect(0, 0, 8, 8)}
	if resp := session.PrintImage(context.Background(), rendered, 1); resp.OK || resp.Error.Code != ErrPrintFailed {
		t.Fatalf("PrintImage() = %#v, want failure", resp)
	}
	if meta := session.Metadata(); meta != nil {
		t.Fatalf("Metadata() = %#v, want disconnected session", meta)
	}
	if backend.connections() != 2 || backend.closed() != 2 {
		t.Fatalf("connections = %d, closed = %d; want 2 and 2", backend.connections(), backend.closed())
	}
}

func TestServiceSerializesPrintsToSamePrinter(t *testing.T) {
	svc := mustService(t)
	backend := &testBackend{slowPrint: true}
	svc.transport = transport.NewManager(backend)
	t.Cleanup(func() { _ = svc.Close() })
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := api.PrintRequest{Printer: api.PrinterSelector{Selector: "b1-round"}, QR: api.QRRequest{Text: "https://example.test/item"}}
			if resp := svc.Print(context.Background(), req); !resp.OK {
				t.Errorf("Print() = %#v", resp)
			}
		}()
	}
	wg.Wait()
	backend.mu.Lock()
	overlapped := backend.overlapped
	backend.mu.Unlock()
	if overlapped {
		t.Fatal("concurrent print jobs overlapped on one physical connection")
	}
}

type testBackend struct {
	mu             sync.Mutex
	connectCount   int
	closeCount     int
	prepareCount   int
	probeCount     int
	failNextPrint  bool
	failEveryPrint bool
	slowPrint      bool
	inFlight       int
	overlapped     bool
}

func (b *testBackend) Connect(context.Context, config.PrinterProfile) (transport.Connection, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connectCount++
	return &testConnection{backend: b}, nil
}

func (b *testBackend) connections() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connectCount
}

func (b *testBackend) closed() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeCount
}

func (b *testBackend) prepared() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.prepareCount
}

func (b *testBackend) probes() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.probeCount
}

type testConnection struct{ backend *testBackend }

func (c *testConnection) Print(context.Context, config.PrinterProfile, transport.Job) error {
	b := c.backend
	b.mu.Lock()
	if b.failEveryPrint || b.failNextPrint {
		b.failNextPrint = false
		b.mu.Unlock()
		return errors.New("device disconnected")
	}
	b.inFlight++
	if b.inFlight > 1 {
		b.overlapped = true
	}
	b.mu.Unlock()
	if b.slowPrint {
		time.Sleep(10 * time.Millisecond)
	}
	b.mu.Lock()
	b.inFlight--
	b.mu.Unlock()
	return nil
}
func (c *testConnection) Prepare(context.Context, config.PrinterProfile) error {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	c.backend.prepareCount++
	return nil
}
func (c *testConnection) Probe(context.Context) error {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	c.backend.probeCount++
	return nil
}
func (*testConnection) Metadata() map[string]any { return map[string]any{"connected": true} }
func (c *testConnection) Close() error {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	c.backend.closeCount++
	return nil
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
