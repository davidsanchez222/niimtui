package tui

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

func TestInvertDocumentChangesRenderedOutput(t *testing.T) {
	doc := label.NewDocument(20, 10)
	normal, err := render.RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument(normal) error = %v", err)
	}
	doc.Inverted = true
	inverted, err := render.RenderDocument(doc)
	if err != nil {
		t.Fatalf("RenderDocument(inverted) error = %v", err)
	}
	if bytes.Equal(normal.PreviewPNG, inverted.PreviewPNG) {
		t.Fatal("inverted render matched normal render")
	}
}

func TestGridIsVisualOnly(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width = 160
	m.Height = 32
	m.reflow()
	m.Grid = true
	canvas := renderCanvas(m)
	if !strings.ContainsRune(canvas, gridHorizontalRune) || !strings.ContainsRune(canvas, gridVerticalRune) {
		t.Fatalf("canvas did not contain grid line runes: %q", canvas)
	}
	m.Grid = false
	withoutGrid, err := render.RenderDocument(m.Document)
	if err != nil {
		t.Fatalf("RenderDocument(without grid) error = %v", err)
	}
	m.Grid = true
	withGrid, err := render.RenderDocument(m.Document)
	if err != nil {
		t.Fatalf("RenderDocument(with grid) error = %v", err)
	}
	if !bytes.Equal(withoutGrid.PreviewPNG, withGrid.PreviewPNG) {
		t.Fatal("grid changed rendered PNG output")
	}
}

func TestExportPNGPromptAndWrite(t *testing.T) {
	m := NewModel(40, 12, "rect", "", PrintConfig{})
	m.beginExportPNGPrompt()
	if !strings.HasPrefix(m.Prompt.Value, "./niimtui-") || !strings.HasSuffix(m.Prompt.Value, ".png") {
		t.Fatalf("default export path = %q", m.Prompt.Value)
	}
	if !strings.ContainsRune(m.Status, promptCursorRune) {
		t.Fatalf("export prompt status = %q, want cursor %q", m.Status, promptCursorRune)
	}
	path := filepath.Join(t.TempDir(), "label.png")
	if !m.exportPNGToPath(path) {
		t.Fatal("exportPNGToPath() = false, want true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(export) error = %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("export is not valid PNG: %v", err)
	}
}

func TestSaveAndLoadDesignPreset(t *testing.T) {
	path := writeDesignFeatureConfig(t)
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{ConfigPath: path}, nil, "")
	m.addTextElement()
	element, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}
	element.Text.Value = "Saved"
	m.Document.UpdateElement(element)
	m.Document.Inverted = true

	if !m.saveDesignPreset("saved-layout") {
		t.Fatal("saveDesignPreset() = false, want true")
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.DesignPresets) != 1 || loaded.DesignPresets[0].Name != "saved-layout" {
		t.Fatalf("design presets = %#v", loaded.DesignPresets)
	}
	m.Document = label.NewDocument(10, 10)
	m.DesignPresets = loaded.DesignPresets
	m.loadNextDesignPreset()
	if !m.Document.Inverted || len(m.Document.Elements) != 1 || m.Document.Elements[0].Text.Value != "Saved" {
		t.Fatalf("loaded document = %#v", m.Document)
	}
}

func writeDesignFeatureConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{
		Server:        config.ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test-token"},
		ActivePrinter: "b1-default",
		Printers: []config.PrinterProfile{{
			Name:          "b1-default",
			Model:         "B1",
			Transport:     "ble",
			DeviceName:    "B1-Test",
			DefaultPreset: "b1-50x30",
		}},
		Presets: []config.LabelPreset{{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-title", MarginsMM: 2}},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return path
}
