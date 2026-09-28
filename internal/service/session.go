package service

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/printtrace"
	"niimtui/internal/render"
	"niimtui/internal/transport"
)

type Session struct {
	service *Service
	printer config.PrinterProfile

	mu   sync.Mutex
	conn transport.Connection
}

func (s *Service) NewSession(selector string) (*Session, error) {
	printer, errResp := s.resolvePrinter(selector)
	if errResp != nil {
		if errResp.Error != nil {
			return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return nil, fmt.Errorf("resolve printer")
	}
	return &Session{service: s, printer: printer}, nil
}

func (s *Session) Printer() config.PrinterProfile {
	return s.printer
}

func (s *Session) Connect(ctx context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		return s.conn.Metadata(), nil
	}
	return s.connectLocked(ctx)
}

func (s *Session) connectLocked(ctx context.Context) (map[string]any, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := s.service.transport.Connect(connectCtx, s.printer)
	if err != nil {
		return nil, fmt.Errorf("connect printer: %w", err)
	}
	if err := conn.Prepare(connectCtx, s.printer); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("prepare printer: %w", err)
	}
	s.conn = conn
	printtrace.Mark(ctx, "BLE connected and ready")
	return conn.Metadata(), nil
}

func (s *Session) closeLocked() error {
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

func (s *Session) PrintImage(ctx context.Context, rendered render.Result, copies int) api.PrintResponse {
	ctx = printtrace.Start(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	printtrace.Mark(ctx, "printer session ready")

	if normalizedCopies(copies) <= 0 {
		return *errorResponse(ErrInvalidRequest, "options.copies must be greater than zero")
	}
	if s.conn == nil {
		return *errorResponse(ErrBLEConnectFailed, "printer is not connected")
	}
	var err error
	rendered, err = render.FitToPrinterWidth(rendered, s.printer.Model)
	if err != nil {
		return *errorResponse(ErrInvalidImage, fmt.Sprintf("fit image to printer: %v", err))
	}
	offsetX, offsetY := render.ModelPrintOffsetMM(s.printer.Model, s.printer.Defaults.OffsetXMM, s.printer.Defaults.OffsetYMM)
	rendered, err = render.ApplyPrintOffset(rendered, offsetX, offsetY)
	if err != nil {
		return *errorResponse(ErrInvalidImage, fmt.Sprintf("offset image for printer: %v", err))
	}
	printtrace.Mark(ctx, "image fitting complete")

	job := transport.Job{Rendered: rendered, Copies: normalizedCopies(copies)}
	err = s.printLocked(ctx, job)
	printtrace.Mark(ctx, "print attempt complete")
	if err != nil {
		printtrace.Mark(ctx, "print failed; reconnecting and retrying")
		firstErr := err
		_ = s.closeLocked()
		if _, reconnectErr := s.connectLocked(ctx); reconnectErr != nil {
			return api.PrintResponse{
				OK:      false,
				Printer: s.printer.Name,
				Copies:  normalizedCopies(copies),
				Error: &api.ErrorBody{
					Code:    ErrBLEConnectFailed,
					Message: fmt.Sprintf("print failed (%v), then reconnect failed: %v", firstErr, reconnectErr),
				},
			}
		}
		err = s.printLocked(ctx, job)
		printtrace.Mark(ctx, "retry complete")
	}
	connectionMeta := map[string]any(nil)
	if s.conn != nil {
		connectionMeta = s.conn.Metadata()
	}
	meta := map[string]any{
		"model":       s.printer.Model,
		"transport":   s.printer.Transport,
		"device_name": s.printer.DeviceName,
		"identifier":  s.printer.Identifier,
		"source":      "image",
		"render": map[string]any{
			"width_px":      rendered.WidthPx,
			"height_px":     rendered.HeightPx,
			"preview_bytes": rendered.PreviewBytes,
		},
		"connection": connectionMeta,
	}
	if err != nil {
		_ = s.closeLocked()
		return api.PrintResponse{
			OK:      false,
			Printer: s.printer.Name,
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
		Printer: s.printer.Name,
		Copies:  normalizedCopies(copies),
		Meta:    meta,
	}
}

func (s *Session) printLocked(ctx context.Context, job transport.Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("printer connection panic: %v\n%s", r, debug.Stack())
		}
	}()
	return s.conn.Print(ctx, s.printer, job)
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closeLocked()
}

func (s *Session) Metadata() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}
	return s.conn.Metadata()
}
