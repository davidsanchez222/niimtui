package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/render"
	"niimtui/internal/server"
	"niimtui/internal/service"
	"niimtui/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runDefaultCommand()
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "print":
		return runPrint(args[1:])
	case "calibrate":
		return runCalibrate(args[1:])
	case "probe":
		return runProbe(args[1:])
	case "scan":
		return runScan(args[1:])
	case "setup":
		return runSetup(args[1:])
	case "printers":
		return runPrinters(args[1:])
	case "presets":
		return runPresets(args[1:])
	case "tui":
		return runTUI(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDefaultCommand() error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check config path: %w", err)
		}
		if err := runSetup(nil); err != nil {
			return err
		}
	}
	return runTUI(nil)
}

func runCalibrate(args []string) error {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	printer := fs.String("printer", "", "printer profile selector")
	presetName := fs.String("preset", "", "label preset")
	copies := fs.Int("copies", 1, "number of copies")
	previewOut := fs.String("preview-out", "", "write calibration preview PNG to this path")
	noPrint := fs.Bool("no-print", false, "render preview only and skip printing; requires --preview-out")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *noPrint && *previewOut == "" {
		return errors.New("-no-print requires -preview-out")
	}

	cfg, err := config.LoadOptional(*configPath)
	if err != nil {
		return err
	}
	printerProfile, _, err := printerForSelector(cfg, *printer)
	if err != nil {
		return err
	}
	preset, err := calibrationPreset(cfg, printerProfile, *presetName)
	if err != nil {
		return err
	}
	rendered, err := render.CalibrationLabel(preset.WidthMM, preset.HeightMM, preset.Shape)
	if err != nil {
		return err
	}
	if *previewOut != "" {
		previewRendered, err := render.FitToPrinterWidth(rendered, printerProfile.Model)
		if err != nil {
			return err
		}
		offsetX, offsetY := render.ModelPrintOffsetMM(printerProfile.Model, printerProfile.Defaults.OffsetXMM, printerProfile.Defaults.OffsetYMM)
		previewRendered, err = render.ApplyPrintOffset(previewRendered, offsetX, offsetY)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*previewOut, previewRendered.PreviewPNG, 0o644); err != nil {
			return fmt.Errorf("write preview: %w", err)
		}
		if *noPrint {
			return nil
		}
	}
	svc, err := service.New(cfg)
	if err != nil {
		return err
	}
	resp := svc.PrintImage(context.Background(), printerProfile.Name, rendered, *copies)
	return printJSON(resp)
}

func calibrationPreset(cfg config.Config, printer config.PrinterProfile, presetName string) (config.LabelPreset, error) {
	presetName = strings.TrimSpace(presetName)
	if presetName == "" {
		return defaultPresetForPrinter(cfg, printer)
	}
	for _, preset := range cfg.Presets {
		if preset.Name == presetName {
			return preset, nil
		}
	}
	return config.LabelPreset{}, fmt.Errorf("unknown preset %q", presetName)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", "config.example.json", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	svc, err := service.New(cfg)
	if err != nil {
		return err
	}

	srv := server.New(cfg, svc)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return srv.Run(ctx)
}

func runPrint(args []string) error {
	fs := flag.NewFlagSet("print", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	printer := fs.String("printer", "", "printer profile selector")
	preset := fs.String("preset", "", "label preset")
	layout := fs.String("layout", string(api.LayoutQROnly), "label layout: qr-only, qr-title, qr-title-subtitle")
	qrText := fs.String("qr-text", "", "text to encode into the QR code")
	imagePath := fs.String("image", "", "PNG image to print directly")
	title := fs.String("title", "", "optional label title")
	subtitle := fs.String("subtitle", "", "optional label subtitle")
	copies := fs.Int("copies", 1, "number of copies")
	previewOut := fs.String("preview-out", "", "write rendered preview PNG to this path")
	noPrint := fs.Bool("no-print", false, "render preview only and skip printing; requires --preview-out")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *imagePath == "" && *qrText == "" {
		return errors.New("-qr-text is required")
	}
	if *noPrint && *previewOut == "" {
		return errors.New("-no-print requires -preview-out")
	}

	cfg, err := config.LoadOptional(*configPath)
	if err != nil {
		return err
	}

	svc, err := service.New(cfg)
	if err != nil {
		return err
	}

	if *imagePath != "" {
		rendered, err := render.PNGFile(*imagePath)
		if err != nil {
			return err
		}
		if *previewOut != "" {
			if err := os.WriteFile(*previewOut, rendered.PreviewPNG, 0o644); err != nil {
				return fmt.Errorf("write preview: %w", err)
			}
			if *noPrint {
				return nil
			}
		}
		resp := svc.PrintImage(context.Background(), *printer, rendered, *copies)
		return printJSON(resp)
	}

	req := api.PrintRequest{
		Printer: api.PrinterSelector{Selector: *printer},
		Label: api.LabelRequest{
			Preset: *preset,
			Layout: api.Layout(*layout),
		},
		QR: api.QRRequest{Text: *qrText},
		Content: api.ContentRequest{
			Title:    *title,
			Subtitle: *subtitle,
		},
		Options: api.PrintOptions{Copies: *copies},
	}

	if *previewOut != "" {
		preview, err := svc.RenderPreview(context.Background(), req)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*previewOut, preview, 0o644); err != nil {
			return fmt.Errorf("write preview: %w", err)
		}
		if *noPrint {
			return nil
		}
	}

	resp := svc.Print(context.Background(), req)

	return printJSON(resp)
}

func runPrinters(args []string) error {
	fs := flag.NewFlagSet("printers", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadOptional(*configPath)
	if err != nil {
		return err
	}

	return printJSON(cfg.Printers)
}

func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	printer := fs.String("printer", "", "printer profile selector")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadOptional(*configPath)
	if err != nil {
		return err
	}

	svc, err := service.New(cfg)
	if err != nil {
		return err
	}

	meta, errResp := svc.Probe(context.Background(), *printer)
	if errResp != nil {
		return printJSON(errResp)
	}

	return printJSON(map[string]any{
		"ok":   true,
		"meta": meta,
	})
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	configPath := fs.String("config", "config.example.json", "path to config JSON")
	transportName := fs.String("transport", "ble", "transport to scan")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	svc, err := service.New(cfg)
	if err != nil {
		return err
	}

	results, err := svc.Scan(context.Background(), *transportName)
	if err != nil {
		return err
	}

	return printJSON(results)
}

func runPresets(args []string) error {
	fs := flag.NewFlagSet("presets", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadOptional(*configPath)
	if err != nil {
		return err
	}

	return printJSON(cfg.Presets)
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	printer := fs.String("printer", "", "printer profile selector")
	widthMM := fs.Float64("width-mm", 0, "label width in millimeters")
	heightMM := fs.Float64("height-mm", 0, "label height in millimeters")
	fontPath := fs.String("font-path", "", "optional TTF/OTF font path for label text rendering")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var svc *service.Service
	var session *service.Session
	var printers []config.PrinterProfile
	var designPresets []config.DesignPreset
	resolvedConfigPath := *configPath
	printerSelector := *printer
	printerProfileName := printerSelector
	shape := "rect"
	printerModel := ""
	deviceName := ""
	identifier := ""
	offsetXMM := 0.0
	offsetYMM := 0.0
	var presets []config.LabelPreset
	activePresetName := ""
	shouldLoadConfig := *widthMM <= 0 || *heightMM <= 0 || *configPath != "" || *printer != ""
	if !shouldLoadConfig {
		if path, err := config.DefaultPath(); err == nil {
			if _, err := os.Stat(path); err == nil {
				shouldLoadConfig = true
			}
		}
	}
	if shouldLoadConfig {
		if resolvedConfigPath == "" {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			resolvedConfigPath = path
		}
		cfg, err := config.LoadOptional(*configPath)
		if err != nil {
			if *widthMM > 0 && *heightMM > 0 && *configPath == "" && *printer == "" {
				return tui.Run(*widthMM, *heightMM, shape, *fontPath, tui.PrintConfig{})
			}
			return err
		}
		presets = cfg.Presets
		printers = cfg.Printers
		designPresets = cfg.DesignPresets
		printerProfile, ok, err := printerForSelector(cfg, printerSelector)
		if err != nil && (*printer != "" || *widthMM <= 0 || *heightMM <= 0) {
			return err
		}
		if ok {
			printerProfileName = printerProfile.Name
			printerSelector = printerProfile.Name
			printerModel = printerProfile.Model
			deviceName = printerProfile.DeviceName
			identifier = printerProfile.Identifier
			offsetXMM = printerProfile.Defaults.OffsetXMM
			offsetYMM = printerProfile.Defaults.OffsetYMM
		}
		if *widthMM <= 0 || *heightMM <= 0 {
			preset, err := defaultPresetForPrinter(cfg, printerProfile)
			if err != nil {
				return err
			}
			activePresetName = preset.Name
			if *widthMM <= 0 {
				*widthMM = preset.WidthMM
			}
			if *heightMM <= 0 {
				*heightMM = preset.HeightMM
			}
			shape = preset.Shape
		}
		svc, err = service.New(cfg)
		if err != nil {
			return err
		}
		if ok {
			session, err = svc.NewSession(printerProfile.Name)
			if err != nil {
				return err
			}
		}
	}
	if *widthMM <= 0 {
		return errors.New("-width-mm is required and must be greater than zero unless setup/default config provides a preset")
	}
	if *heightMM <= 0 {
		return errors.New("-height-mm is required and must be greater than zero unless setup/default config provides a preset")
	}

	var newSession tui.PrinterSessionFactory
	if svc != nil {
		newSession = func(selector string) (tui.PrinterSession, error) {
			return svc.NewSession(selector)
		}
	}
	return tui.RunWithPresets(*widthMM, *heightMM, shape, *fontPath, tui.PrintConfig{Session: session, NewSession: newSession, ConfigPath: resolvedConfigPath, Printers: printers, DesignPresets: designPresets, Printer: printerProfileName, Model: printerModel, DeviceName: deviceName, Identifier: identifier, OffsetXMM: offsetXMM, OffsetYMM: offsetYMM, Copies: 1}, presets, activePresetName)
}

func defaultPreset(cfg config.Config, printerSelector string) (config.LabelPreset, error) {
	printer, _, err := printerForSelector(cfg, printerSelector)
	if err != nil {
		return config.LabelPreset{}, err
	}
	return defaultPresetForPrinter(cfg, printer)
}

func printerForSelector(cfg config.Config, printerSelector string) (config.PrinterProfile, bool, error) {
	printerSelector = strings.TrimSpace(printerSelector)
	if printerSelector == "" {
		if cfg.ActivePrinter != "" {
			for _, candidate := range cfg.Printers {
				if candidate.Name == cfg.ActivePrinter {
					return candidate, true, nil
				}
			}
			return config.PrinterProfile{}, false, fmt.Errorf("active printer profile %q not found", cfg.ActivePrinter)
		}
		if len(cfg.Printers) != 1 {
			return config.PrinterProfile{}, false, errors.New("-printer is required when config has multiple printers and no active_printer")
		}
		return cfg.Printers[0], true, nil
	}
	for _, candidate := range cfg.Printers {
		if candidate.Name == printerSelector {
			return candidate, true, nil
		}
	}
	return config.PrinterProfile{}, false, fmt.Errorf("unknown printer profile %q", printerSelector)
}

func defaultPresetForPrinter(cfg config.Config, printer config.PrinterProfile) (config.LabelPreset, error) {
	for _, preset := range cfg.Presets {
		if preset.Name == printer.DefaultPreset {
			return preset, nil
		}
	}
	return config.LabelPreset{}, fmt.Errorf("printer %q references unknown default preset %q", printer.Name, printer.DefaultPreset)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `niimtui

Usage:
  niimtui
  niimtui serve --config ./config.example.json
  niimtui setup
  niimtui calibrate --config ./config.example.json --printer b1-round --preset b1-50x30 --preview-out ./calibration.png
  niimtui print --config ./config.example.json --printer d110-desk --image ./testlabels/preview.png
  niimtui probe --config ./config.example.json --printer d110-desk
  niimtui scan --config ./config.example.json --transport ble
  niimtui printers --config ./config.example.json
  niimtui presets --config ./config.example.json
  niimtui tui --width-mm 50 --height-mm 30 --font-path /path/to/font.ttf
`)
}
