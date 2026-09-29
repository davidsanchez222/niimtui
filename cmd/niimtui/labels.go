package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/render"
	"niimtui/internal/service"
)

type commandExit struct {
	code    int
	message string
}

type stockMismatchError struct{ message string }

func (e *stockMismatchError) Error() string { return e.message }

func (e *commandExit) Error() string { return e.message }

type setValues map[string]string

func (s *setValues) String() string { return "name=value (repeatable)" }

func (s *setValues) Set(input string) error {
	name, value, ok := strings.Cut(input, "=")
	if !ok || !config.ValidBindingName(name) {
		return fmt.Errorf("--set expects name=value with a valid binding name")
	}
	if *s == nil {
		*s = make(setValues)
	}
	if _, exists := (*s)[name]; exists {
		return fmt.Errorf("duplicate --set %q", name)
	}
	(*s)[name] = value
	return nil
}

func runDesigns(args []string) error {
	fs := flag.NewFlagSet("designs", flag.ContinueOnError)
	path := fs.String("config", "", "path to config JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("designs does not accept arguments")
	}
	cfg, err := config.LoadOptional(*path)
	if err != nil {
		return err
	}
	type bindingInfo struct {
		Name     string            `json:"name"`
		Type     label.ElementType `json:"type"`
		Required bool              `json:"required"`
	}
	type designInfo struct {
		Name     string        `json:"name"`
		WidthMM  float64       `json:"width_mm"`
		HeightMM float64       `json:"height_mm"`
		Shape    string        `json:"shape"`
		Bindings []bindingInfo `json:"bindings"`
	}
	result := make([]designInfo, 0, len(cfg.DesignPresets))
	for _, preset := range cfg.DesignPresets {
		item := designInfo{Name: preset.Name, WidthMM: preset.Document.WidthMM, HeightMM: preset.Document.HeightMM, Shape: preset.Document.Shape, Bindings: []bindingInfo{}}
		for _, binding := range preset.Bindings {
			for _, element := range preset.Document.Elements {
				if element.ID == binding.ElementID {
					item.Bindings = append(item.Bindings, bindingInfo{Name: binding.Name, Type: element.Type, Required: binding.Required})
					break
				}
			}
		}
		result = append(result, item)
	}
	return printJSON(result)
}

func runPrint(args []string) error   { return runLabelCommand("print", args) }
func runPreview(args []string) error { return runLabelCommand("preview", args) }

func runLabelCommand(action string, args []string) error {
	err := executeLabelCommand(action, args)
	if err == nil {
		return nil
	}
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	var exit *commandExit
	if errors.As(err, &exit) {
		return err
	}
	code := service.ErrInvalidRequest
	var mismatch *stockMismatchError
	if errors.As(err, &mismatch) {
		code = "LABEL_STOCK_MISMATCH"
	}
	if err := printJSON(api.PrintResponse{OK: false, Error: &api.ErrorBody{Code: code, Message: err.Error()}}); err != nil {
		return err
	}
	return &commandExit{code: 1}
}

func executeLabelCommand(action string, args []string) error {
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	path := fs.String("config", "", "path to config JSON")
	printerName := fs.String("printer", "", "printer profile (defaults to active printer)")
	presetName := fs.String("preset", "", "installed label roll (defaults to printer preset)")
	designName := fs.String("design", "", "saved TUI design name")
	qrText := fs.String("qr", "", "QR contents for a quick label")
	imagePath := fs.String("image", "", "PNG to print or preview")
	title := fs.String("title", "", "quick label title")
	subtitle := fs.String("subtitle", "", "quick label subtitle")
	copies := fs.Int("copies", 1, "number of copies (1-20)")
	out := fs.String("out", "", "preview output PNG path")
	var values setValues
	fs.Var(&values, "set", "set a named design binding (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	sources := 0
	for _, source := range []string{*designName, *qrText, *imagePath} {
		if source != "" {
			sources++
		}
	}
	if sources != 1 {
		return errors.New("choose exactly one of --design, --qr, or --image")
	}
	if *copies < 1 || *copies > 20 {
		return errors.New("--copies must be between 1 and 20")
	}
	if action == "preview" && *out == "" {
		return errors.New("preview requires --out")
	}
	if action == "print" && *out != "" {
		return errors.New("--out is only available with preview")
	}
	if action == "preview" && *copies != 1 {
		return errors.New("--copies is only available with print")
	}
	if *designName == "" && len(values) != 0 {
		return errors.New("--set requires --design")
	}
	if *qrText == "" && (*title != "" || *subtitle != "") {
		return errors.New("--title and --subtitle require --qr")
	}
	if *subtitle != "" && *title == "" {
		return errors.New("--subtitle requires --title")
	}
	if *imagePath != "" && *presetName != "" {
		return errors.New("--preset does not apply to an image")
	}

	cfg, err := config.LoadOptional(*path)
	if err != nil {
		return err
	}
	printer, _, err := printerForSelector(cfg, *printerName)
	if err != nil {
		return err
	}
	preset := printer.DefaultPreset
	if *presetName != "" {
		preset = *presetName
	}
	var stock config.LabelPreset
	if *imagePath == "" {
		found := false
		for _, candidate := range cfg.Presets {
			if candidate.Name == preset {
				stock, found = candidate, true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown label preset %q", preset)
		}
	}
	ctx := context.Background()
	var rendered render.Result
	if *qrText != "" {
		layout := api.LayoutQROnly
		if *title != "" {
			layout = api.LayoutQRTitle
		}
		if *subtitle != "" {
			layout = api.LayoutQRTitleSubtitle
		}
		req := api.PrintRequest{Printer: api.PrinterSelector{Selector: printer.Name}, Label: api.LabelRequest{Preset: preset, Layout: layout}, QR: api.QRRequest{Text: *qrText}, Content: api.ContentRequest{Title: *title, Subtitle: *subtitle}, Options: api.PrintOptions{Copies: *copies}}
		svc, err := service.New(cfg)
		if err != nil {
			return err
		}
		defer svc.Close()
		if action == "print" {
			return reportPrint(svc.Print(ctx, req))
		}
		png, err := svc.RenderPreview(ctx, req)
		if err != nil {
			return err
		}
		return writePreview(*out, png, printer.Name, preset)
	}
	if *imagePath != "" {
		rendered, err = render.PNGFile(*imagePath)
		if err != nil {
			return err
		}
		preset = ""
	} else {
		var design *config.DesignPreset
		for i := range cfg.DesignPresets {
			if cfg.DesignPresets[i].Name == *designName {
				design = &cfg.DesignPresets[i]
				break
			}
		}
		if design == nil {
			return fmt.Errorf("unknown design %q; run `niimtui designs`", *designName)
		}
		var doc label.Document
		doc, err = design.Bind(values)
		if err != nil {
			return err
		}
		if action == "print" && !config.MatchesStock(doc, stock) {
			return &stockMismatchError{message: fmt.Sprintf("design %q does not match selected stock: design %.3gx%.3g mm %s; preset %q %.3gx%.3g mm %s (select matching stock with --preset)", design.Name, doc.WidthMM, doc.HeightMM, doc.Shape, stock.Name, stock.WidthMM, stock.HeightMM, stock.Shape)}
		}
		rendered, err = render.RenderDocument(doc)
		if err != nil {
			return err
		}
	}
	if action == "print" {
		svc, err := service.New(cfg)
		if err != nil {
			return err
		}
		defer svc.Close()
		return reportPrint(svc.PrintImage(ctx, printer.Name, rendered, *copies))
	}
	rendered, err = render.FitToPrinterWidth(rendered, printer.Model)
	if err != nil {
		return err
	}
	x, y := render.ModelPrintOffsetMM(printer.Model, printer.Defaults.OffsetXMM, printer.Defaults.OffsetYMM)
	rendered, err = render.ApplyPrintOffset(rendered, x, y)
	if err != nil {
		return err
	}
	return writePreview(*out, rendered.PreviewPNG, printer.Name, preset)
}

func reportPrint(resp api.PrintResponse) error {
	if err := printJSON(resp); err != nil {
		return err
	}
	if !resp.OK {
		return &commandExit{code: 1}
	}
	return nil
}

func writePreview(path string, png []byte, printer, preset string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create preview directory: %w", err)
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return fmt.Errorf("write preview: %w", err)
	}
	return printJSON(map[string]any{"ok": true, "path": path, "printer": printer, "preset": preset})
}
