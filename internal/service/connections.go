package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"niimtui/internal/config"
	"niimtui/internal/printtrace"
	"niimtui/internal/transport"
)

// printerConnection serializes access to a physical printer, including its
// connection establishment. Multiple profiles may refer to the same device.
type printerConnection struct {
	mu     sync.Mutex
	conn   transport.Connection
	closed bool
}

func printerKey(printer config.PrinterProfile) string {
	for _, value := range []string{printer.Identifier, printer.Address, printer.DeviceName} {
		if value != "" {
			return printer.Transport + ":" + strings.ToLower(value)
		}
	}
	return printer.Transport + ":" + printer.Name
}

func (s *Service) connectionFor(printer config.PrinterProfile) (*printerConnection, error) {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()
	if s.closed {
		return nil, errors.New("print service is closed")
	}
	key := printerKey(printer)
	if slot := s.connections[key]; slot != nil {
		return slot, nil
	}
	slot := &printerConnection{}
	s.connections[key] = slot
	return slot, nil
}

// connectLocked assumes the caller holds slot.mu.
func (s *Service) connectLocked(ctx context.Context, slot *printerConnection, printer config.PrinterProfile) error {
	if slot.conn != nil {
		return nil
	}
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := s.transport.Connect(connectCtx, printer)
	if err != nil {
		return fmt.Errorf("connect printer: %w", err)
	}
	slot.conn = conn
	printtrace.Mark(ctx, "BLE connected")
	return nil
}

func (s *Service) send(ctx context.Context, printer config.PrinterProfile, job transport.Job) (map[string]any, string, error) {
	slot, err := s.connectionFor(printer)
	if err != nil {
		return nil, ErrBLEConnectFailed, err
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil, ErrBLEConnectFailed, errors.New("print service is closed")
	}
	printtrace.Mark(ctx, "printer slot acquired")
	if slot.conn != nil {
		printtrace.Mark(ctx, "reusing BLE connection")
	}
	if err := s.connectLocked(ctx, slot, printer); err != nil {
		return nil, ErrBLEConnectFailed, err
	}
	err = slot.conn.Print(ctx, printer, job)
	printtrace.Mark(ctx, "print command complete")
	meta := slot.conn.Metadata()
	if err != nil {
		_ = slot.conn.Close()
		slot.conn = nil
		return meta, ErrPrintFailed, err
	}
	return meta, "", nil
}

// Close releases every connection held by the long-running service. A
// one-shot CLI can also call it before exiting.
func (s *Service) Close() error {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()
	s.closed = true
	var errs []error
	for _, slot := range s.connections {
		slot.mu.Lock()
		slot.closed = true
		if slot.conn != nil {
			errs = append(errs, slot.conn.Close())
			slot.conn = nil
		}
		slot.mu.Unlock()
	}
	return errors.Join(errs...)
}
