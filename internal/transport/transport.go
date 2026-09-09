package transport

import (
	"context"
	"fmt"

	"niimcli/internal/config"
	"niimcli/internal/render"
)

type Backend interface {
	Connect(ctx context.Context, printer config.PrinterProfile) (Connection, error)
}

type Connection interface {
	Print(ctx context.Context, printer config.PrinterProfile, job Job) error
	Probe(ctx context.Context) error
	Close() error
	Metadata() map[string]any
}

type Job struct {
	Rendered render.Result
	Copies   int
}

type ScanResult struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int16  `json:"rssi"`
}

type Manager struct {
	ble Backend
}

func NewManager(ble Backend) *Manager {
	return &Manager{ble: ble}
}

func (m *Manager) Connect(ctx context.Context, printer config.PrinterProfile) (Connection, error) {
	switch printer.Transport {
	case "ble":
		if m.ble == nil {
			return nil, fmt.Errorf("ble backend not configured")
		}
		return m.ble.Connect(ctx, printer)
	default:
		return nil, fmt.Errorf("unsupported transport %q", printer.Transport)
	}
}

func (m *Manager) Scan(ctx context.Context, transportName string) ([]ScanResult, error) {
	switch transportName {
	case "", "ble":
		scanner, ok := m.ble.(interface {
			Scan(context.Context) ([]ScanResult, error)
		})
		if !ok || m.ble == nil {
			return nil, fmt.Errorf("ble backend does not support scanning")
		}
		return scanner.Scan(ctx)
	default:
		return nil, fmt.Errorf("unsupported transport %q", transportName)
	}
}

func (m *Manager) ScanStream(ctx context.Context, transportName string) (<-chan ScanResult, <-chan error, error) {
	switch transportName {
	case "", "ble":
		scanner, ok := m.ble.(interface {
			ScanStream(context.Context) (<-chan ScanResult, <-chan error, error)
		})
		if !ok || m.ble == nil {
			return nil, nil, fmt.Errorf("ble backend does not support streaming scans")
		}
		return scanner.ScanStream(ctx)
	default:
		return nil, nil, fmt.Errorf("unsupported transport %q", transportName)
	}
}
