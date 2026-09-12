package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"niimtui/internal/api"
	"niimtui/internal/config"
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

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, err := s.service.transport.Connect(connectCtx, s.printer)
	if err != nil {
		return nil, fmt.Errorf("connect printer: %w", err)
	}
	s.conn = conn
	return conn.Metadata(), nil
}

func (s *Session) PrintImage(ctx context.Context, rendered render.Result, copies int) api.PrintResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	if normalizedCopies(copies) <= 0 {
		return *errorResponse(ErrInvalidRequest, "options.copies must be greater than zero")
	}
	if s.conn == nil {
		return *errorResponse(ErrBLEConnectFailed, "printer is not connected")
	}

	err := s.conn.Print(ctx, s.printer, transport.Job{Rendered: rendered, Copies: normalizedCopies(copies)})
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
		"connection": s.conn.Metadata(),
	}
	if err != nil {
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

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

func (s *Session) Metadata() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}
	return s.conn.Metadata()
}
