package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"niimcli/internal/api"
	"niimcli/internal/config"
	"niimcli/internal/server"
	"niimcli/internal/service"
	"niimcli/internal/tui"
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
	configPath := fs.String("config", "config.example.json", "path to config JSON")
	printer := fs.String("printer", "", "printer profile selector")
	preset := fs.String("preset", "", "label preset")
	layout := fs.String("layout", string(api.LayoutQROnly), "label layout: qr-only, qr-title, qr-title-subtitle")
	qrText := fs.String("qr-text", "", "text to encode into the QR code")
	title := fs.String("title", "", "optional label title")
	subtitle := fs.String("subtitle", "", "optional label subtitle")
	copies := fs.Int("copies", 1, "number of copies")
	previewOut := fs.String("preview-out", "", "write rendered preview PNG to this path")
	noPrint := fs.Bool("no-print", false, "render preview only and skip printing; requires --preview-out")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *qrText == "" {
		return errors.New("-qr-text is required")
	}
	if *noPrint && *previewOut == "" {
		return errors.New("-no-print requires -preview-out")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	svc, err := service.New(cfg)
	if err != nil {
		return err
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
	configPath := fs.String("config", "config.example.json", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	return printJSON(cfg.Printers)
}

func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	configPath := fs.String("config", "config.example.json", "path to config JSON")
	printer := fs.String("printer", "", "printer profile selector")
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
	configPath := fs.String("config", "config.example.json", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	return printJSON(cfg.Presets)
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	widthMM := fs.Float64("width-mm", 0, "label width in millimeters")
	heightMM := fs.Float64("height-mm", 0, "label height in millimeters")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *widthMM <= 0 {
		return errors.New("-width-mm is required and must be greater than zero")
	}
	if *heightMM <= 0 {
		return errors.New("-height-mm is required and must be greater than zero")
	}

	return tui.Run(*widthMM, *heightMM)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `niimcli

Usage:
  niimcli serve --config ./config.example.json
  niimcli print --config ./config.example.json --printer d110-desk --preset d110-12x40 --image ./label.png --preview-out ./preview.png
  niimcli probe --config ./config.example.json --printer d110-desk
  niimcli scan --config ./config.example.json --transport ble
  niimcli printers --config ./config.example.json
  niimcli presets --config ./config.example.json
  niimcli tui --width-mm 50 --height-mm 30
`)
}
