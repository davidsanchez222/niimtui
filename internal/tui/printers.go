package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
)

func (m *Model) switchPrinter(delta int) tea.Cmd {
	if len(m.Print.Printers) == 0 || delta == 0 {
		m.setStatus("No installed printers available.")
		return nil
	}
	if len(m.Print.Printers) == 1 {
		m.setStatus("Only one printer is installed: %s.", m.Print.Printers[0].Name)
		return nil
	}
	index := activePrinterIndex(m.Print.Printers, m.Print.Printer)
	if index < 0 {
		index = 0
	} else {
		index = (index + delta) % len(m.Print.Printers)
		if index < 0 {
			index += len(m.Print.Printers)
		}
	}
	return m.applyPrinter(m.Print.Printers[index])
}

func activePrinterIndex(printers []config.PrinterProfile, name string) int {
	name = strings.TrimSpace(name)
	for i, printer := range printers {
		if printer.Name == name {
			return i
		}
	}
	return -1
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

	status := fmt.Sprintf("Switched to printer %s (%s).", printer.Name, printer.Model)
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
