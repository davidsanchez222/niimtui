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
		printUsage()
		return errors.New("missing command")
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "print":
		return runPrint(args[1:])
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
	printerSelector := *printer
	shape := "rect"
	printerModel := ""
	shouldLoadConfig := *widthMM <= 0 || *heightMM <= 0 || *configPath != "" || *printer != ""
	if !shouldLoadConfig {
		if path, err := config.DefaultPath(); err == nil {
			if _, err := os.Stat(path); err == nil {
				shouldLoadConfig = true
			}
		}
	}
	if shouldLoadConfig {
		cfg, err := config.LoadOptional(*configPath)
		if err != nil {
			if *widthMM > 0 && *heightMM > 0 && *configPath == "" && *printer == "" {
				return tui.Run(*widthMM, *heightMM, shape, *fontPath, tui.PrintConfig{})
			}
			return err
		}
		printerProfile, ok, err := printerForSelector(cfg, printerSelector)
		if err != nil && (*printer != "" || *widthMM <= 0 || *heightMM <= 0) {
			return err
		}
		if ok {
			printerModel = printerProfile.Model
		}
		if *widthMM <= 0 || *heightMM <= 0 {
			preset, err := defaultPresetForPrinter(cfg, printerProfile)
			if err != nil {
				return err
			}
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
	}
	if *widthMM <= 0 {
		return errors.New("-width-mm is required and must be greater than zero unless setup/default config provides a preset")
	}
	if *heightMM <= 0 {
		return errors.New("-height-mm is required and must be greater than zero unless setup/default config provides a preset")
	}

	return tui.Run(*widthMM, *heightMM, shape, *fontPath, tui.PrintConfig{Service: svc, Printer: printerSelector, Model: printerModel, Copies: 1})
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
		if len(cfg.Printers) != 1 {
			return config.PrinterProfile{}, false, errors.New("-printer is required when config has multiple printers")
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
  niimtui serve --config ./config.example.json
  niimtui setup
  niimtui print --config ./config.example.json --printer d110-desk --image ./testlabels/preview.png
  niimtui probe --config ./config.example.json --printer d110-desk
  niimtui scan --config ./config.example.json --transport ble
  niimtui printers --config ./config.example.json
  niimtui presets --config ./config.example.json
  niimtui tui --width-mm 50 --height-mm 30 --font-path /path/to/font.ttf
`)
}
