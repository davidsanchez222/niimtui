package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"niimcli/internal/config"
	"niimcli/internal/service"
	"niimcli/internal/transport"
)

var (
	setupTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	setupHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	setupCursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	setupSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
)

const setupInitialScanWindow = 2 * time.Second

var knownNiimbotModels = []string{
	"M2",
	"M3",
	"N1",
	"B21 Pro",
	"B1",
	"B4",
	"B2 Pro",
	"B1 Pro",
	"B2",
	"B21",
	"B3S",
	"K3",
	"D11",
	"D110",
	"D101",
}

func runSetup(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("setup does not accept arguments")
	}

	path, err := config.DefaultPath()
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, setupTitleStyle.Render("niimcli setup"))
	fmt.Fprintln(os.Stdout, setupHintStyle.Render("Configure your default printer and label roll."))
	fmt.Fprintln(os.Stdout)

	if _, err := os.Stat(path); err == nil {
		overwrite := false
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Config already exists at %s. Overwrite it?", path)).
			Affirmative("Overwrite").
			Negative("Cancel").
			Value(&overwrite).
			Run(); err != nil {
			return err
		}
		if !overwrite {
			return errors.New("setup cancelled")
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check config path: %w", err)
	}

	model := "B1"
	if err := huh.NewSelect[string]().
		Title("Which Niimbot printer do you have?").
		Options(
			huh.NewOption("B1", "B1"),
			huh.NewOption("D110", "D110"),
		).
		Value(&model).
		Run(); err != nil {
		return err
	}

	presets := setupPresetsForModel(model)
	if len(presets) == 0 {
		return fmt.Errorf("no built-in presets for model %s", model)
	}

	presetName := presets[0].Name
	presetOptions := make([]huh.Option[string], 0, len(presets))
	for _, preset := range presets {
		label := fmt.Sprintf("%s (%.0fx%.0fmm %s)", preset.Name, preset.WidthMM, preset.HeightMM, preset.Shape)
		presetOptions = append(presetOptions, huh.NewOption(label, preset.Name))
	}
	if err := huh.NewSelect[string]().
		Title("Which label roll is installed?").
		Options(presetOptions...).
		Value(&presetName).
		Run(); err != nil {
		return err
	}

	useScan := true
	if err := huh.NewConfirm().
		Title("Scan for your printer over Bluetooth?").
		Description("Manual entry is available if scanning is not convenient.").
		Affirmative("Scan").
		Negative("Manual").
		Value(&useScan).
		Run(); err != nil {
		return err
	}

	printer, err := setupPrinterProfile(model, presetName, useScan)
	if err != nil {
		return err
	}

	cfg, err := config.DefaultConfig(printer, presets)
	if err != nil {
		return err
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%s %s\n", setupTitleStyle.Render("Saved config:"), path)
	fmt.Fprintln(os.Stdout, setupHintStyle.Render("You can now try `niimcli tui` or `niimcli print --image ./testlabels/preview.png`."))
	return nil
}

func setupPrinterProfile(model, presetName string, useScan bool) (config.PrinterProfile, error) {
	printer := config.PrinterProfile{
		Name:          defaultPrinterName(model),
		Model:         model,
		Transport:     "ble",
		DefaultPreset: presetName,
		Defaults: config.PrinterDefaults{
			Density: 3,
			Rotate:  0,
		},
	}

	if useScan {
		selected, ok, err := chooseScannedDevice()
		if err != nil {
			fallback := false
			if promptErr := huh.NewConfirm().
				Title(fmt.Sprintf("Bluetooth scan failed: %v", err)).
				Description("Continue with manual printer details?").
				Affirmative("Manual").
				Negative("Cancel").
				Value(&fallback).
				Run(); promptErr != nil {
				return config.PrinterProfile{}, promptErr
			}
			if !fallback {
				return config.PrinterProfile{}, err
			}
		} else if ok {
			printer.DeviceName = selected.Name
			printer.Identifier = selected.Address
			if printer.DeviceName == "" {
				printer.DeviceName = selected.Address
			}
			return printer, nil
		}
	}

	deviceName := ""
	identifier := ""
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Printer Bluetooth device name").
			Description("Example: B1-I427031488").
			Value(&deviceName),
		huh.NewInput().
			Title("Bluetooth identifier or address").
			Description("Optional if the device name is reliable.").
			Value(&identifier),
	)).Run(); err != nil {
		return config.PrinterProfile{}, err
	}
	printer.DeviceName = strings.TrimSpace(deviceName)
	printer.Identifier = strings.TrimSpace(identifier)
	if printer.DeviceName == "" && printer.Identifier == "" {
		return config.PrinterProfile{}, fmt.Errorf("device name or identifier is required")
	}
	return printer, nil
}

func chooseScannedDevice() (transport.ScanResult, bool, error) {
	svc, err := service.New(config.Config{})
	if err != nil {
		return transport.ScanResult{}, false, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results, errs, err := svc.ScanStream(ctx, "ble")
	if err != nil {
		return transport.ScanResult{}, false, err
	}

	p := tea.NewProgram(newScanPicker(results, errs))
	model, err := p.Run()
	if err != nil {
		return transport.ScanResult{}, false, err
	}
	picker, ok := model.(scanPicker)
	if !ok {
		return transport.ScanResult{}, false, fmt.Errorf("unexpected scan picker result")
	}
	if picker.err != nil {
		return transport.ScanResult{}, false, picker.err
	}
	return picker.selected, picker.selected.Address != "", nil
}

type scanResultMsg transport.ScanResult

type scanErrMsg struct{ err error }

type initialScanDoneMsg struct{}

type scanPicker struct {
	results     <-chan transport.ScanResult
	errs        <-chan error
	devices     map[string]transport.ScanResult
	named       []transport.ScanResult
	unknown     []transport.ScanResult
	cursor      int
	ready       bool
	showUnknown bool

	selected transport.ScanResult
	err      error
}

type scanOptionKind int

const (
	scanOptionDevice scanOptionKind = iota
	scanOptionUnknownSummary
	scanOptionManual
)

type scanOption struct {
	kind   scanOptionKind
	line   string
	device transport.ScanResult
}

func newScanPicker(results <-chan transport.ScanResult, errs <-chan error) scanPicker {
	return scanPicker{results: results, errs: errs, devices: make(map[string]transport.ScanResult)}
}

func (m scanPicker) Init() tea.Cmd {
	return tea.Batch(waitScanResult(m.results), waitScanError(m.errs), tea.Tick(setupInitialScanWindow, func(time.Time) tea.Msg {
		return initialScanDoneMsg{}
	}))
}

func (m scanPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanResultMsg:
		result := transport.ScanResult(msg)
		if result.Address != "" {
			m.devices[result.Address] = result
			m.sortDevices()
			m.clampCursor()
		}
		return m, waitScanResult(m.results)
	case scanErrMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m, nil
	case initialScanDoneMsg:
		m.ready = true
		return m, nil
	case tea.KeyMsg:
		maxCursor := m.optionCount() - 1
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "u":
			m.showUnknown = !m.showUnknown
			m.clampCursor()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < maxCursor {
				m.cursor++
			}
		case "enter":
			options := m.options()
			if m.cursor < len(options) {
				switch option := options[m.cursor]; option.kind {
				case scanOptionDevice:
					m.selected = option.device
					return m, tea.Quit
				case scanOptionUnknownSummary:
					m.showUnknown = true
					return m, nil
				case scanOptionManual:
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m scanPicker) View() string {
	var b strings.Builder
	b.WriteString(setupTitleStyle.Render("Scanning for devices"))
	b.WriteString("\n")
	if !m.ready {
		b.WriteString(setupHintStyle.Render("Building the initial list for 2 seconds. New devices will keep appearing after that."))
		b.WriteString("\n\n")
	} else {
		unknownHint := "u show unknown"
		if m.showUnknown {
			unknownHint = "u hide unknown"
		}
		b.WriteString(setupHintStyle.Render("Select your printer. New devices appear live. " + unknownHint + " • q skip"))
		b.WriteString("\n\n")
	}

	options := m.options()
	for i, option := range options {
		cursor := "  "
		line := option.line
		if i == m.cursor {
			cursor = setupCursorStyle.Render("> ")
			line = setupSelectedStyle.Render(line)
		}
		b.WriteString(cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *scanPicker) sortDevices() {
	m.named = m.named[:0]
	m.unknown = m.unknown[:0]
	for _, result := range m.devices {
		if strings.TrimSpace(result.Name) == "" {
			m.unknown = append(m.unknown, result)
			continue
		}
		m.named = append(m.named, result)
	}
	sort.SliceStable(m.named, func(i, j int) bool {
		leftRank, leftKnown := knownNiimbotRank(m.named[i].Name)
		rightRank, rightKnown := knownNiimbotRank(m.named[j].Name)
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown && leftRank != rightRank {
			return leftRank < rightRank
		}
		left := strings.ToLower(m.named[i].Name)
		right := strings.ToLower(m.named[j].Name)
		if left == right {
			return m.named[i].Address < m.named[j].Address
		}
		return left < right
	})
	sort.SliceStable(m.unknown, func(i, j int) bool {
		return m.unknown[i].Address < m.unknown[j].Address
	})
}

func (m scanPicker) optionCount() int {
	return len(m.options())
}

func (m scanPicker) options() []scanOption {
	options := make([]scanOption, 0, len(m.named)+len(m.unknown)+2)
	for _, result := range m.named {
		options = append(options, scanOption{kind: scanOptionDevice, line: scanDeviceLine(result), device: result})
	}
	if len(m.unknown) > 0 {
		if m.showUnknown {
			for _, result := range m.unknown {
				options = append(options, scanOption{kind: scanOptionDevice, line: scanDeviceLine(result), device: result})
			}
		} else {
			options = append(options, scanOption{kind: scanOptionUnknownSummary, line: fmt.Sprintf("Unknown devices hidden (%d) - press u to show", len(m.unknown))})
		}
	}
	options = append(options, scanOption{kind: scanOptionManual, line: "Enter manually"})
	return options
}

func (m scanPicker) optionLines() []string {
	options := m.options()
	lines := make([]string, 0, len(options))
	for _, option := range options {
		lines = append(lines, option.line)
	}
	return lines
}

func (m *scanPicker) clampCursor() {
	if m.cursor >= m.optionCount() {
		m.cursor = max(m.optionCount()-1, 0)
	}
}

func scanDeviceLine(result transport.ScanResult) string {
	name := strings.TrimSpace(result.Name)
	if name == "" {
		name = "Unknown device"
	}
	return fmt.Sprintf("%-24s %s  RSSI %d", name, result.Address, result.RSSI)
}

func knownNiimbotRank(name string) (int, bool) {
	name = strings.TrimSpace(name)
	for i, model := range knownNiimbotModels {
		if hasModelPrefix(name, model) {
			return i, true
		}
	}
	return 0, false
}

func hasModelPrefix(name, model string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	model = strings.ToLower(strings.TrimSpace(model))
	if name == model {
		return true
	}
	if !strings.HasPrefix(name, model) {
		return false
	}
	next := name[len(model):]
	if next == "" {
		return true
	}
	switch next[0] {
	case ' ', '-', '_':
		return true
	default:
		return false
	}
}

func waitScanResult(results <-chan transport.ScanResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-results
		if !ok {
			return nil
		}
		return scanResultMsg(result)
	}
}

func waitScanError(errs <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-errs
		if !ok {
			return scanErrMsg{}
		}
		return scanErrMsg{err: err}
	}
}

func defaultPrinterName(model string) string {
	switch strings.ToUpper(strings.TrimSpace(model)) {
	case "B1":
		return "b1-default"
	case "D110":
		return "d110-default"
	default:
		return strings.ToLower(strings.TrimSpace(model)) + "-default"
	}
}

func setupPresetsForModel(model string) []config.LabelPreset {
	switch strings.ToUpper(strings.TrimSpace(model)) {
	case "B1":
		return []config.LabelPreset{
			{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-title-subtitle", MarginsMM: 2},
			{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-title", MarginsMM: 2},
			{Name: "b1-50x80", WidthMM: 50, HeightMM: 80, Shape: "rect", Layout: "qr-title-subtitle", MarginsMM: 2},
		}
	case "D110":
		return []config.LabelPreset{
			{Name: "d110-12x40", WidthMM: 40, HeightMM: 12, Shape: "rect", Layout: "qr-only", MarginsMM: 1},
		}
	default:
		return nil
	}
}
