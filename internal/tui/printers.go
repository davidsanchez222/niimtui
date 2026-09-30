package tui

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
	"niimtui/internal/transport"
)

type printerScanMsg struct {
	Seq     int
	Results []transport.ScanResult
	Err     error
}

func printerScanCmd(discover PrinterDiscovery, ctx context.Context, seq int) tea.Cmd {
	return func() tea.Msg {
		results, err := discover(ctx)
		return printerScanMsg{Seq: seq, Results: results, Err: err}
	}
}

func detectedPrinters(printers []config.PrinterProfile, results []transport.ScanResult) []config.PrinterProfile {
	matched := make([]config.PrinterProfile, 0, len(printers))
	names := make(map[string]int, len(printers))
	for _, printer := range printers {
		if printer.Transport == "ble" && printer.DeviceName != "" {
			names[printer.DeviceName]++
		}
	}
	for _, printer := range printers {
		if printer.Transport != "ble" {
			continue
		}
		for _, result := range results {
			if (printer.Identifier != "" && strings.EqualFold(printer.Identifier, result.Address)) ||
				(printer.Address != "" && strings.EqualFold(printer.Address, result.Address)) ||
				(printer.DeviceName != "" && names[printer.DeviceName] == 1 && printer.DeviceName == result.Name) {
				matched = append(matched, printer)
				break
			}
		}
	}
	return matched
}

func chooseDetectedPrinter(printers []config.PrinterProfile, preferred string, explicit bool) (config.PrinterProfile, bool) {
	for _, printer := range printers {
		if printer.Name == preferred {
			return printer, true
		}
	}
	if explicit || len(printers) != 1 {
		return config.PrinterProfile{}, false
	}
	return printers[0], true
}

func (m *Model) startPrinterScan() tea.Cmd {
	if m.Print.Discover == nil {
		m.setStatus("Printer discovery is unavailable.")
		return nil
	}
	if m.Connection == ConnectionConnected || m.Connection == ConnectionConnecting {
		m.setStatus("Disconnect before rescanning for a printer.")
		return nil
	}
	if m.Discovering {
		m.setStatus("Printer scan already running.")
		return nil
	}
	if m.scanCancel != nil {
		m.scanCancel()
	}
	parent := m.Print.DiscoveryContext
	if parent == nil {
		parent = context.Background()
	}
	m.scanCtx, m.scanCancel = context.WithCancel(parent)
	m.ScanSeq++
	m.Discovering = true
	m.DetectedNames = nil
	m.setStatus("Looking for configured printers...")
	return printerScanCmd(m.Print.Discover, m.scanCtx, m.ScanSeq)
}

func (m *Model) handlePrinterScan(msg printerScanMsg) tea.Cmd {
	if msg.Seq != m.ScanSeq || !m.Discovering {
		return nil
	}
	m.Discovering = false
	if m.scanCancel != nil {
		m.scanCancel()
		m.scanCancel = nil
	}
	if m.PendingConnect {
		m.PendingConnect = false
		return m.connectSelectedPrinter()
	}
	if msg.Err != nil {
		m.DetectedNames = nil
		m.setStatus("Printer discovery failed: %v. Select a printer and press c to connect manually.", msg.Err)
		return nil
	}
	found := detectedPrinters(m.Print.Printers, msg.Results)
	m.DetectedNames = make(map[string]bool, len(found))
	for _, printer := range found {
		m.DetectedNames[printer.Name] = true
	}
	selected, ok := chooseDetectedPrinter(found, m.Print.PreferredPrinter, m.Print.ExplicitPrinter)
	if !ok {
		if m.Print.ExplicitPrinter {
			m.setStatus("Selected printer not advertising. Press Tab to manage printers or c to connect manually.")
		} else if len(found) == 0 {
			m.setStatus("No configured printers advertising. Press r in the printer panel to rescan or c to connect manually.")
		} else {
			m.setStatus("Multiple printers available; select one in the printer panel.")
		}
		return nil
	}
	if m.Print.Session != nil || m.Connection == ConnectionConnecting || m.Connection == ConnectionConnected {
		return nil
	}
	unchanged := reflect.DeepEqual(m.Document, m.startupDocument)
	m.setPrinterProfile(selected, unchanged)
	if unchanged {
		m.initHistory()
		m.cleanDocument = cloneDocument(m.Document)
	}
	return m.connectSelectedPrinter()
}

func (m *Model) applyPrinter(printer config.PrinterProfile) tea.Cmd {
	m.ConnectSeq++
	if m.Discovering && m.scanCancel != nil {
		m.scanCancel()
	}
	m.closePrinterSession()
	m.Print.Session = nil
	m.setPrinterProfile(printer, true)
	m.Print.PreferredPrinter = printer.Name
	m.Print.ExplicitPrinter = false
	m.Connection = ConnectionDisconnected
	m.ConnectErr = ""
	m.ConnectMeta = nil
	status := fmt.Sprintf("Switched to %s.", printerDisplayName(printer))
	if len(m.Presets) == 0 {
		status += " No installed label rolls match this printer."
	}
	if err := saveActivePrinter(m.Print.ConfigPath, printer.Name); err != nil {
		status += " Failed to save active printer: " + err.Error()
	}
	m.setStatus("%s", status)
	if m.Discovering {
		m.PendingConnect = true
		m.setStatus("Stopping discovery before connecting to %s...", printer.Name)
		return nil
	}
	return m.connectSelectedPrinter()
}

func (m *Model) setPrinterProfile(printer config.PrinterProfile, resize bool) {
	m.Print.Printer = printer.Name
	for i, candidate := range m.Print.Printers {
		if candidate.Name == printer.Name {
			m.SidebarPrinter = i
			break
		}
	}
	m.Print.Model = printer.Model
	m.Print.DeviceName = printer.DeviceName
	m.Print.Identifier = printer.Identifier
	m.Print.OffsetXMM = printer.Defaults.OffsetXMM
	m.Print.OffsetYMM = printer.Defaults.OffsetYMM
	m.Presets = presetsForPrinter(m.AllPresets, printer.Model)
	if resize {
		m.Preset = activePresetIndex(m.Presets, printer.DefaultPreset, m.Document)
		if m.Preset < 0 && len(m.Presets) > 0 {
			m.Preset = 0
		}
		if m.Preset >= 0 {
			m.applyPreset(m.Preset)
		}
	} else {
		m.Preset = activePresetIndex(m.Presets, "", m.Document)
	}
	m.SidebarRoll = max(0, m.Preset)
}

func (m *Model) connectSelectedPrinter() tea.Cmd {
	if m.Print.Session == nil && m.Print.NewSession != nil {
		session, err := m.Print.NewSession(m.Print.Printer)
		if err != nil {
			m.Connection = ConnectionDisconnected
			m.ConnectErr = err.Error()
			m.setStatus("Create printer session: %v", err)
			return nil
		}
		m.Print.Session = session
	}
	if m.Print.Session == nil {
		m.setStatus("Printer connection unavailable.")
		return nil
	}
	m.Connection = ConnectionConnecting
	m.ConnectSeq++
	m.ConnectErr = ""
	m.setStatus("Connecting to %s...", m.Print.Printer)
	return connectPrinterCmd(m.Print.Session, m.ConnectSeq)
}

func printerDisplayName(printer config.PrinterProfile) string {
	name := strings.TrimSpace(printer.DeviceName)
	if name == "" {
		name = strings.TrimSpace(printer.Identifier)
	}
	if name == "" {
		name = strings.TrimSpace(printer.Address)
	}
	if name == "" {
		name = strings.TrimSpace(printer.Name)
	}
	model := strings.TrimSpace(printer.Model)
	if model == "" {
		return name
	}
	if name == "" {
		return model
	}
	return fmt.Sprintf("%s (%s)", name, model)
}

func saveActivePrinter(path, printerName string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		defaultPath, err := config.DefaultPath()
		if err != nil {
			return err
		}
		path = defaultPath
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cfg.ActivePrinter = printerName
	return config.Save(path, cfg)
}
