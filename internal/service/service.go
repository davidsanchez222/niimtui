package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/render"
	"niimtui/internal/transport"
	bletransport "niimtui/internal/transport/ble"
)

const (
	ErrUnauthorized        = "UNAUTHORIZED"
	ErrInvalidRequest      = "INVALID_REQUEST"
	ErrInvalidImage        = "INVALID_IMAGE"
	ErrPrinterNotFound     = "PRINTER_NOT_FOUND"
	ErrPresetNotFound      = "PRESET_NOT_FOUND"
	ErrRenderFailed        = "RENDER_FAILED"
	ErrBLEConnectFailed    = "BLE_CONNECT_FAILED"
	ErrPrintFailed         = "PRINT_FAILED"
	ErrPrintNotImplemented = "PRINT_NOT_IMPLEMENTED"
)

type Service struct {
	cfg            config.Config
	printersByName map[string]config.PrinterProfile
	presetsByName  map[string]config.LabelPreset
	transport      *transport.Manager
}

func New(cfg config.Config) (*Service, error) {
	s := &Service{
		cfg:            cfg,
		printersByName: make(map[string]config.PrinterProfile, len(cfg.Printers)),
		presetsByName:  make(map[string]config.LabelPreset, len(cfg.Presets)),
		transport:      transport.NewManager(bletransport.New()),
	}
	for _, printer := range cfg.Printers {
		s.printersByName[printer.Name] = printer
	}
	for _, preset := range cfg.Presets {
		s.presetsByName[preset.Name] = preset
	}
	return s, nil
}

func (s *Service) Printers() []config.PrinterProfile {
	return s.cfg.Printers
}

func (s *Service) Presets() []config.LabelPreset {
	return s.cfg.Presets
}

func (s *Service) Scan(ctx context.Context, transportName string) ([]transport.ScanResult, error) {
	return s.transport.Scan(ctx, transportName)
}

func (s *Service) ScanStream(ctx context.Context, transportName string) (<-chan transport.ScanResult, <-chan error, error) {
	return s.transport.ScanStream(ctx, transportName)
}

func (s *Service) Probe(ctx context.Context, selector string) (map[string]any, *api.PrintResponse) {
	printer, errResp := s.resolvePrinter(selector)
	if errResp != nil {
		return nil, errResp
	}

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := s.transport.Connect(connectCtx, printer)
	if err != nil {
		return nil, errorResponse(ErrBLEConnectFailed, fmt.Sprintf("connect printer: %v", err))
	}
	defer conn.Close()

	probeCtx, probeCancel := context.WithTimeout(ctx, 20*time.Second)
	defer probeCancel()
	if err := conn.Probe(probeCtx); err != nil {
		return nil, errorResponse(ErrPrintFailed, fmt.Sprintf("probe printer: %v", err))
	}

	return map[string]any{
		"printer": map[string]any{
			"name":        printer.Name,
			"model":       printer.Model,
			"transport":   printer.Transport,
			"device_name": printer.DeviceName,
			"identifier":  printer.Identifier,
		},
		"connection": conn.Metadata(),
	}, nil
}

func (s *Service) Print(ctx context.Context, req api.PrintRequest) api.PrintResponse {
	printer, preset, rendered, errResp := s.preparePrint(req)
	if errResp != nil {
		return *errResp
	}

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := s.transport.Connect(connectCtx, printer)
	if err != nil {
		return *errorResponse(ErrBLEConnectFailed, fmt.Sprintf("connect printer: %v", err))
	}
	defer conn.Close()

	err = conn.Print(ctx, printer, transport.Job{Rendered: rendered, Copies: normalizedCopies(req.Options.Copies)})
	meta := map[string]any{
		"model":       printer.Model,
		"transport":   printer.Transport,
		"device_name": printer.DeviceName,
		"identifier":  printer.Identifier,
		"layout":      normalizedLayout(req.Label.Layout),
		"render": map[string]any{
			"width_px":         rendered.WidthPx,
			"height_px":        rendered.HeightPx,
			"printable_width":  rendered.PrintablePx.Dx(),
			"printable_height": rendered.PrintablePx.Dy(),
			"shape":            rendered.Shape,
			"rotation":         rendered.Rotation,
			"preview_bytes":    rendered.PreviewBytes,
		},
		"connection": conn.Metadata(),
	}
	if err != nil {
		return api.PrintResponse{
			OK:      false,
			Printer: printer.Name,
			Preset:  preset.Name,
			Copies:  normalizedCopies(req.Options.Copies),
			Error: &api.ErrorBody{
				Code:    ErrPrintFailed,
				Message: err.Error(),
			},
			Meta: meta,
		}
	}

	return api.PrintResponse{
		OK:      true,
		Printer: printer.Name,
		Preset:  preset.Name,
		Copies:  normalizedCopies(req.Options.Copies),
		Meta:    meta,
	}
}

func (s *Service) PrintImage(ctx context.Context, selector string, rendered render.Result, copies int) api.PrintResponse {
	printer, errResp := s.resolvePrinter(selector)
	if errResp != nil {
		return *errResp
	}
	if normalizedCopies(copies) <= 0 {
		return *errorResponse(ErrInvalidRequest, "options.copies must be greater than zero")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := s.transport.Connect(connectCtx, printer)
	if err != nil {
		return *errorResponse(ErrBLEConnectFailed, fmt.Sprintf("connect printer: %v", err))
	}
	defer conn.Close()

	err = conn.Print(ctx, printer, transport.Job{Rendered: rendered, Copies: normalizedCopies(copies)})
	meta := map[string]any{
		"model":       printer.Model,
		"transport":   printer.Transport,
		"device_name": printer.DeviceName,
		"identifier":  printer.Identifier,
		"source":      "image",
		"render": map[string]any{
			"width_px":      rendered.WidthPx,
			"height_px":     rendered.HeightPx,
			"preview_bytes": rendered.PreviewBytes,
		},
		"connection": conn.Metadata(),
	}
	if err != nil {
		return api.PrintResponse{
			OK:      false,
			Printer: printer.Name,
			Copies:  normalizedCopies(copies),
			Error: &api.ErrorBody{
				Code:    ErrPrintFailed,
				Message: err.Error(),
			},
			Meta: meta,
		}
	}

	return api.PrintResponse{
		OK:      true,
		Printer: printer.Name,
		Copies:  normalizedCopies(copies),
		Meta:    meta,
	}
}

func (s *Service) RenderPreview(_ context.Context, req api.PrintRequest) ([]byte, error) {
	_, _, rendered, errResp := s.preparePrint(req)
	if errResp != nil {
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}
	return rendered.PreviewPNG, nil
}

func (s *Service) PreparePreview(_ context.Context, req api.PrintRequest) ([]byte, *api.PrintResponse) {
	_, _, rendered, errResp := s.preparePrint(req)
	if errResp != nil {
		return nil, errResp
	}
	return rendered.PreviewPNG, nil
}

func (s *Service) preparePrint(req api.PrintRequest) (config.PrinterProfile, config.LabelPreset, render.Result, *api.PrintResponse) {
	printer, preset, errResp := s.validateRequest(req)
	if errResp != nil {
		return config.PrinterProfile{}, config.LabelPreset{}, render.Result{}, errResp
	}

	rendered, err := render.QRLabel(req, printer, preset)
	if err != nil {
		return config.PrinterProfile{}, config.LabelPreset{}, render.Result{}, errorResponse(ErrRenderFailed, fmt.Sprintf("render label: %v", err))
	}

	return printer, preset, rendered, nil
}

func (s *Service) validateRequest(req api.PrintRequest) (config.PrinterProfile, config.LabelPreset, *api.PrintResponse) {
	selector := strings.TrimSpace(req.Printer.Selector)
	printer, errResp := s.resolvePrinter(selector)
	if errResp != nil {
		return config.PrinterProfile{}, config.LabelPreset{}, errResp
	}

	presetName := strings.TrimSpace(req.Label.Preset)
	if presetName == "" {
		presetName = printer.DefaultPreset
	}

	preset, ok := s.presetsByName[presetName]
	if !ok {
		return config.PrinterProfile{}, config.LabelPreset{}, errorResponse(ErrPresetNotFound, fmt.Sprintf("unknown preset %q", presetName))
	}

	if normalizedCopies(req.Options.Copies) <= 0 {
		return config.PrinterProfile{}, config.LabelPreset{}, errorResponse(ErrInvalidRequest, "options.copies must be greater than zero")
	}

	if strings.TrimSpace(req.QR.Text) == "" {
		return config.PrinterProfile{}, config.LabelPreset{}, errorResponse(ErrInvalidRequest, "qr.text is required")
	}

	layout := normalizedLayout(req.Label.Layout)
	if layout != api.LayoutQROnly && layout != api.LayoutQRTitle && layout != api.LayoutQRTitleSubtitle {
		return config.PrinterProfile{}, config.LabelPreset{}, errorResponse(ErrInvalidRequest, "label.layout is not supported")
	}

	if len(req.Content.Title) > 120 || len(req.Content.Subtitle) > 120 {
		return config.PrinterProfile{}, config.LabelPreset{}, errorResponse(ErrInvalidRequest, "content fields must be 120 characters or fewer")
	}

	return printer, preset, nil
}

func (s *Service) resolvePrinter(selector string) (config.PrinterProfile, *api.PrintResponse) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		if len(s.cfg.Printers) == 1 {
			return s.cfg.Printers[0], nil
		}
		return config.PrinterProfile{}, errorResponse(ErrInvalidRequest, "printer.selector is required")
	}

	printer, ok := s.printersByName[selector]
	if !ok {
		return config.PrinterProfile{}, errorResponse(ErrPrinterNotFound, fmt.Sprintf("unknown printer profile %q", selector))
	}
	return printer, nil
}

func normalizedLayout(layout api.Layout) api.Layout {
	if layout == "" {
		return api.LayoutQROnly
	}
	return layout
}

func normalizedCopies(copies int) int {
	if copies == 0 {
		return 1
	}
	if copies < 0 {
		return copies
	}
	if copies > 20 {
		return 20
	}
	return copies
}

func errorResponse(code, message string) *api.PrintResponse {
	return &api.PrintResponse{
		OK: false,
		Error: &api.ErrorBody{
			Code:    code,
			Message: message,
		},
	}
}
