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
	return printJSON(designInfos(cfg))
}

func designInfos(cfg config.Config) []designInfo {
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
	return result
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
	if err := printJSON(api.PrintResponse{OK: false, Error: errorBody(err)}); err != nil {
		return err
	}
	return &commandExit{code: 1}
}

func errorBody(err error) *api.ErrorBody {
	code := service.ErrInvalidRequest
	var mismatch *stockMismatchError
	if errors.As(err, &mismatch) {
		code = "LABEL_STOCK_MISMATCH"
	}
	return &api.ErrorBody{Code: code, Message: err.Error()}
}

func executeLabelCommand(action string, args []string) error {
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	path := fs.String("config", "", "path to config JSON")
	var spec labelSpec
	fs.StringVar(&spec.Printer, "printer", "", "printer profile (defaults to active printer)")
	fs.StringVar(&spec.Preset, "preset", "", "installed label roll (defaults to printer preset)")
	fs.StringVar(&spec.Design, "design", "", "saved TUI design name")
	fs.StringVar(&spec.QR, "qr", "", "QR contents for a quick label")
	fs.StringVar(&spec.Image, "image", "", "PNG to print or preview")
	fs.StringVar(&spec.Title, "title", "", "quick label title")
	fs.StringVar(&spec.Subtitle, "subtitle", "", "quick label subtitle")
	fs.IntVar(&spec.Copies, "copies", 1, "number of copies (1-20)")
	batch := fs.String("batch", "", "JSONL file of --set values, one label per line (- for stdin); requires --design")
	out := fs.String("out", "", "preview output PNG path (a directory with --batch)")
	var values setValues
	fs.Var(&values, "set", "set a named design binding (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	spec.Values = values
	if action == "preview" && *out == "" {
		return errors.New("preview requires --out")
	}
	if action == "print" && *out != "" {
		return errors.New("--out is only available with preview")
	}
	if action == "preview" && spec.Copies != 1 {
		return errors.New("--copies is only available with print")
	}
	if *batch != "" {
		if spec.Design == "" {
			return errors.New("--batch requires --design")
		}
		if len(values) != 0 {
			return errors.New("--batch and --set cannot be combined; put the values in the batch rows")
		}
		rows, err := readBatchFile(*batch)
		if err != nil {
			return err
		}
		cfg, err := config.LoadOptional(*path)
		if err != nil {
			return err
		}
		return runBatch(action, cfg, spec, rows, *out)
	}

	cfg, err := config.LoadOptional(*path)
	if err != nil {
		return err
	}
	job, err := prepareLabel(cfg, spec, action == "print")
	if err != nil {
		return err
	}
	svc, err := service.New(cfg)
	if err != nil {
		return err
	}
	defer svc.Close()
	ctx := context.Background()
	if action == "print" {
		return reportPrint(job.print(ctx, svc))
	}
	png, err := job.previewPNG(ctx, svc)
	if err != nil {
		return err
	}
	return writePreview(*out, png, job.printer.Name, job.preset)
}

// labelSpec is one label request, whether it came from CLI flags, a batch row
// or an MCP tool call. Exactly one of Design, QR or Image is set.
type labelSpec struct {
	Printer  string
	Preset   string
	Design   string
	Values   map[string]string
	QR       string
	Title    string
	Subtitle string
	Image    string
	Copies   int
}

// labelJob is a validated, rendered label: printing it can only fail at the printer.
type labelJob struct {
	printer  config.PrinterProfile
	preset   string
	copies   int
	qr       *api.PrintRequest // quick QR labels are laid out by the service
	rendered render.Result
}

func prepareLabel(cfg config.Config, spec labelSpec, forPrint bool) (labelJob, error) {
	sources := 0
	for _, source := range []string{spec.Design, spec.QR, spec.Image} {
		if source != "" {
			sources++
		}
	}
	if sources != 1 {
		return labelJob{}, errors.New("choose exactly one of --design, --qr, or --image")
	}
	if spec.Copies == 0 {
		spec.Copies = 1
	}
	if spec.Copies < 1 || spec.Copies > 20 {
		return labelJob{}, errors.New("--copies must be between 1 and 20")
	}
	if spec.Design == "" && len(spec.Values) != 0 {
		return labelJob{}, errors.New("--set requires --design")
	}
	if spec.QR == "" && (spec.Title != "" || spec.Subtitle != "") {
		return labelJob{}, errors.New("--title and --subtitle require --qr")
	}
	if spec.Subtitle != "" && spec.Title == "" {
		return labelJob{}, errors.New("--subtitle requires --title")
	}
	if spec.Image != "" && spec.Preset != "" {
		return labelJob{}, errors.New("--preset does not apply to an image")
	}

	printer, _, err := printerForSelector(cfg, spec.Printer)
	if err != nil {
		return labelJob{}, err
	}
	job := labelJob{printer: printer, preset: printer.DefaultPreset, copies: spec.Copies}
	if spec.Preset != "" {
		job.preset = spec.Preset
	}
	if spec.Image != "" {
		job.preset = ""
		job.rendered, err = render.PNGFile(spec.Image)
		return job, err
	}
	var stock config.LabelPreset
	found := false
	for _, candidate := range cfg.Presets {
		if candidate.Name == job.preset {
			stock, found = candidate, true
			break
		}
	}
	if !found {
		return labelJob{}, fmt.Errorf("unknown label preset %q", job.preset)
	}
	if spec.QR != "" {
		layout := api.LayoutQROnly
		if spec.Title != "" {
			layout = api.LayoutQRTitle
		}
		if spec.Subtitle != "" {
			layout = api.LayoutQRTitleSubtitle
		}
		job.qr = &api.PrintRequest{Printer: api.PrinterSelector{Selector: printer.Name}, Label: api.LabelRequest{Preset: job.preset, Layout: layout}, QR: api.QRRequest{Text: spec.QR}, Content: api.ContentRequest{Title: spec.Title, Subtitle: spec.Subtitle}, Options: api.PrintOptions{Copies: spec.Copies}}
		return job, nil
	}
	var design *config.DesignPreset
	for i := range cfg.DesignPresets {
		if cfg.DesignPresets[i].Name == spec.Design {
			design = &cfg.DesignPresets[i]
			break
		}
	}
	if design == nil {
		return labelJob{}, fmt.Errorf("unknown design %q; run `niimtui designs`", spec.Design)
	}
	doc, err := design.Bind(spec.Values)
	if err != nil {
		return labelJob{}, err
	}
	if forPrint && !config.MatchesStock(doc, stock) {
		return labelJob{}, &stockMismatchError{message: fmt.Sprintf("design %q does not match selected stock: design %.3gx%.3g mm %s; preset %q %.3gx%.3g mm %s (select matching stock with --preset)", design.Name, doc.WidthMM, doc.HeightMM, doc.Shape, stock.Name, stock.WidthMM, stock.HeightMM, stock.Shape)}
	}
	job.rendered, err = render.RenderDocument(doc)
	return job, err
}

func (j labelJob) print(ctx context.Context, svc *service.Service) api.PrintResponse {
	if j.qr != nil {
		return svc.Print(ctx, *j.qr)
	}
	return svc.PrintImage(ctx, j.printer.Name, j.rendered, j.copies)
}

// previewPNG renders exactly what print would send, fitted and offset for the printer.
func (j labelJob) previewPNG(ctx context.Context, svc *service.Service) ([]byte, error) {
	if j.qr != nil {
		return svc.RenderPreview(ctx, *j.qr)
	}
	rendered, err := render.FitToPrinterWidth(j.rendered, j.printer.Model)
	if err != nil {
		return nil, err
	}
	x, y := render.ModelPrintOffsetMM(j.printer.Model, j.printer.Defaults.OffsetXMM, j.printer.Defaults.OffsetYMM)
	rendered, err = render.ApplyPrintOffset(rendered, x, y)
	if err != nil {
		return nil, err
	}
	return rendered.PreviewPNG, nil
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
	if err := writePNG(path, png); err != nil {
		return err
	}
	return printJSON(map[string]any{"ok": true, "path": path, "printer": printer, "preset": preset})
}

func writePNG(path string, png []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create preview directory: %w", err)
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return fmt.Errorf("write preview: %w", err)
	}
	return nil
}
