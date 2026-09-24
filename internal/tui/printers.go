package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
)

func (m *Model) switchPrinterIndex(index int) tea.Cmd {
	if index < 0 || index >= len(m.Print.Printers) {
		m.setStatus("No printer assigned to %d.", index+1)
		return nil
	}
	printer := m.Print.Printers[index]
	if printer.Name == m.Print.Printer {
		m.setStatus("Printer already active: %s.", printerDisplayName(printer))
		return nil
	}
	return m.applyPrinter(printer)
}

func (m *Model) applyPrinter(printer config.PrinterProfile) tea.Cmd {
	m.closePrinterSession()
	m.Print.Session = nil
	m.Print.Printer = printer.Name
	m.Print.Model = printer.Model
	m.Print.DeviceName = printer.DeviceName
	m.Print.Identifier = printer.Identifier
	m.Print.OffsetXMM = printer.Defaults.OffsetXMM
	m.Print.OffsetYMM = printer.Defaults.OffsetYMM
	m.Presets = presetsForPrinter(m.AllPresets, printer.Model)
	m.Preset = activePresetIndex(m.Presets, printer.DefaultPreset, m.Document)
	if m.Preset < 0 && len(m.Presets) > 0 {
		m.Preset = 0
	}
	if m.Preset >= 0 {
		m.applyPreset(m.Preset)
	}
	m.Connection = ConnectionUnavailable
	m.ConnectErr = ""
	m.ConnectMeta = nil

	status := fmt.Sprintf("Switched to %s.", printerDisplayName(printer))
	if len(m.Presets) == 0 {
		status += " No installed label rolls match this printer."
	}
	if err := saveActivePrinter(m.Print.ConfigPath, printer.Name); err != nil {
		status += " Failed to save active printer: " + err.Error()
	}
	if m.Print.NewSession == nil {
		m.setStatus("%s", status)
		return nil
	}
	session, err := m.Print.NewSession(printer.Name)
	if err != nil {
		m.Connection = ConnectionDisconnected
		m.ConnectErr = err.Error()
		m.setStatus("%s Failed to create printer session: %v", status, err)
		return nil
	}
	m.Print.Session = session
	m.Connection = ConnectionConnecting
	m.setStatus("%s Connecting...", status)
	return connectPrinterCmd(session)
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
