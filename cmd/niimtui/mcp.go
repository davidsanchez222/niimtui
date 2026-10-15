package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/davidsanchez222/niimtui/internal/api"
	"github.com/davidsanchez222/niimtui/internal/config"
	"github.com/davidsanchez222/niimtui/internal/service"
)

const mcpInstructions = `niimtui prints labels on a Niimbot printer over Bluetooth, typically QR labels for Homebox items.
Workflow: list_designs, then preview_label to check the layout, then print_label (or print_labels for many items).
A design's bindings are its fillable fields; pass them in "values". With no suitable design, use "qr" with an optional title and subtitle.
Printing uses physical label stock, so preview first when the layout is new to you.`

type labelInput struct {
	Design   string            `json:"design,omitempty" jsonschema:"saved design name from list_designs"`
	Values   map[string]string `json:"values,omitempty" jsonschema:"design binding values by name, e.g. {\"url\": \"https://homebox.example/item/123\", \"title\": \"Garage Bin 4\"}"`
	QR       string            `json:"qr,omitempty" jsonschema:"QR contents for a quick label; use instead of design"`
	Title    string            `json:"title,omitempty" jsonschema:"quick label title; requires qr"`
	Subtitle string            `json:"subtitle,omitempty" jsonschema:"quick label subtitle; requires title"`
	Printer  string            `json:"printer,omitempty" jsonschema:"printer profile name; defaults to the active printer"`
	Preset   string            `json:"preset,omitempty" jsonschema:"installed label roll; defaults to the printer's roll"`
	Copies   int               `json:"copies,omitempty" jsonschema:"copies to print, 1-20, default 1"`
}

func (in labelInput) spec() labelSpec {
	return labelSpec{Printer: in.Printer, Preset: in.Preset, Design: in.Design, Values: in.Values, QR: in.QR, Title: in.Title, Subtitle: in.Subtitle, Copies: in.Copies}
}

type batchInput struct {
	Design  string              `json:"design" jsonschema:"saved design name from list_designs"`
	Rows    []map[string]string `json:"rows" jsonschema:"one object of binding values per label, printed in order"`
	Printer string              `json:"printer,omitempty" jsonschema:"printer profile name; defaults to the active printer"`
	Preset  string              `json:"preset,omitempty" jsonschema:"installed label roll; defaults to the printer's roll"`
	Copies  int                 `json:"copies,omitempty" jsonschema:"copies of each label, 1-20, default 1"`
}

type printerSummary struct {
	Name          string `json:"name"`
	Model         string `json:"model"`
	DefaultPreset string `json:"default_preset"`
}

type presetSummary struct {
	Name     string  `json:"name"`
	WidthMM  float64 `json:"width_mm"`
	HeightMM float64 `json:"height_mm"`
	Shape    string  `json:"shape"`
}

type printersResult struct {
	ActivePrinter string           `json:"active_printer,omitempty"`
	Printers      []printerSummary `json:"printers"`
	Presets       []presetSummary  `json:"presets"`
}

// mcpServer reloads the config on every call, so designs saved in the TUI show
// up without a restart, but keeps one service so the BLE connection is reused.
type mcpServer struct {
	configPath string

	mu     sync.Mutex
	svc    *service.Service
	svcKey string
}

func runMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	s := &mcpServer{configPath: *configPath}
	defer s.close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.server().Run(ctx, &mcp.StdioTransport{})
}

func (s *mcpServer) server() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "niimtui", Version: version}, &mcp.ServerOptions{Instructions: mcpInstructions})
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	notDestructive := false
	printing := &mcp.ToolAnnotations{DestructiveHint: &notDestructive}
	mcp.AddTool(server, &mcp.Tool{Name: "list_designs", Description: "List saved label designs and the bindings each one accepts.", Annotations: readOnly}, s.listDesigns)
	mcp.AddTool(server, &mcp.Tool{Name: "list_printers", Description: "List configured printers and installed label rolls.", Annotations: readOnly}, s.listPrinters)
	mcp.AddTool(server, &mcp.Tool{Name: "preview_label", Description: "Render a label as a PNG image exactly as it would print, without printing.", Annotations: readOnly}, s.previewLabel)
	mcp.AddTool(server, &mcp.Tool{Name: "print_label", Description: "Print one label on the printer.", Annotations: printing}, s.printLabel)
	mcp.AddTool(server, &mcp.Tool{Name: "print_labels", Description: "Print many labels from one design, one per row. Every row is validated before anything prints; printing stops at the first printer error.", Annotations: printing}, s.printLabels)
	return server
}

func (s *mcpServer) loadConfig() (config.Config, error) {
	return config.LoadOptional(s.configPath)
}

func (s *mcpServer) service(cfg config.Config) (*service.Service, error) {
	key, err := json.Marshal([]any{cfg.Printers, cfg.Presets})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.svc != nil && s.svcKey == string(key) {
		return s.svc, nil
	}
	if s.svc != nil {
		s.svc.Close()
	}
	s.svc, err = service.New(cfg)
	s.svcKey = string(key)
	return s.svc, err
}

func (s *mcpServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.svc != nil {
		s.svc.Close()
	}
}

func (s *mcpServer) listDesigns(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, err
	}
	return nil, designInfos(cfg), nil
}

func (s *mcpServer) listPrinters(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, err
	}
	result := printersResult{ActivePrinter: cfg.ActivePrinter, Printers: []printerSummary{}, Presets: []presetSummary{}}
	for _, p := range cfg.Printers {
		result.Printers = append(result.Printers, printerSummary{Name: p.Name, Model: p.Model, DefaultPreset: p.DefaultPreset})
	}
	for _, p := range cfg.Presets {
		result.Presets = append(result.Presets, presetSummary{Name: p.Name, WidthMM: p.WidthMM, HeightMM: p.HeightMM, Shape: p.Shape})
	}
	return nil, result, nil
}

func (s *mcpServer) previewLabel(ctx context.Context, _ *mcp.CallToolRequest, in labelInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, err
	}
	job, err := prepareLabel(cfg, in.spec(), false)
	if err != nil {
		return nil, nil, err
	}
	svc, err := s.service(cfg)
	if err != nil {
		return nil, nil, err
	}
	png, err := job.previewPNG(ctx, svc)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.ImageContent{Data: png, MIMEType: "image/png"},
		&mcp.TextContent{Text: fmt.Sprintf("printer %s, roll %s", job.printer.Name, job.preset)},
	}}, nil, nil
}

func (s *mcpServer) printLabel(ctx context.Context, _ *mcp.CallToolRequest, in labelInput) (*mcp.CallToolResult, any, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, err
	}
	job, err := prepareLabel(cfg, in.spec(), true)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, api.PrintResponse{OK: false, Error: errorBody(err)}, nil
	}
	svc, err := s.service(cfg)
	if err != nil {
		return nil, nil, err
	}
	resp := job.print(ctx, svc)
	resp.Meta = nil // connection and render details cost tokens and rarely help an agent
	return &mcp.CallToolResult{IsError: !resp.OK}, resp, nil
}

func (s *mcpServer) printLabels(ctx context.Context, _ *mcp.CallToolRequest, in batchInput) (*mcp.CallToolResult, any, error) {
	if len(in.Rows) == 0 {
		return nil, nil, errors.New("rows is empty")
	}
	if len(in.Rows) > maxBatchRows {
		return nil, nil, fmt.Errorf("more than %d rows; split the batch", maxBatchRows)
	}
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, nil, err
	}
	svc, err := s.service(cfg)
	if err != nil {
		return nil, nil, err
	}
	result := printBatch(ctx, svc, cfg, labelSpec{Printer: in.Printer, Preset: in.Preset, Design: in.Design, Copies: in.Copies}, in.Rows)
	return &mcp.CallToolResult{IsError: !result.OK}, result, nil
}
