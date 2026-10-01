package tui

import (
	"bytes"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

func TestGalleryBuildsRenderableStarterLayoutsAndSavedDesigns(t *testing.T) {
	stocks := []config.LabelPreset{
		{Name: "b1-round", WidthMM: 50, HeightMM: 50, Shape: "round"},
		{Name: "d110-small", WidthMM: 40, HeightMM: 12, Shape: "rect"},
	}
	saved := label.NewDocument(50, 50)
	saved.Shape = "round"
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{DesignPresets: []config.DesignPreset{{Name: "my design", Document: saved}}}, stocks, "b1-round")
	items := m.galleryItems()
	if len(items) < 3 || !items[len(items)-1].Saved || items[len(items)-1].Name != "my design" {
		t.Fatalf("gallery items = %#v", items)
	}
	headers := []string{}
	for _, row := range m.galleryRows() {
		if row.Header {
			headers = append(headers, row.Group.title())
		}
	}
	if strings.Join(headers, ",") != "B1,D110,Custom" {
		t.Fatalf("gallery headers = %v", headers)
	}
	for _, item := range items {
		if item.Group == galleryD110 && strings.Contains(strings.ToLower(item.Name), "title") {
			t.Fatalf("small roll has clipped text template: %q", item.Name)
		}
		for _, element := range item.Document.Elements {
			if !item.Saved && element.Text != nil && element.Text.FontSize != 40 {
				t.Fatalf("starter font size = %v for %q", element.Text.FontSize, item.Name)
			}
		}
		result, err := render.RenderDocument(item.Document)
		if err != nil {
			t.Fatalf("render %q: %v", item.Name, err)
		}
		if _, err := png.Decode(bytes.NewReader(result.PreviewPNG)); err != nil {
			t.Fatalf("invalid preview for %q: %v", item.Name, err)
		}
	}
}

func TestGalleryGroupsCollapseWithoutLosingSavedDesigns(t *testing.T) {
	stock := config.LabelPreset{Name: "b1-round", WidthMM: 50, HeightMM: 50, Shape: "round"}
	doc := label.NewDocument(50, 50)
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{DesignPresets: []config.DesignPreset{{Name: "my box", Document: doc}}}, []config.LabelPreset{stock}, stock.Name)
	m.Tab = tabGallery
	m.GalleryIndex = galleryRowIndex(t, m, "b1-round / QR")
	m.handleGalleryKey(testKey("h"))
	if !m.GalleryCollapsed[galleryB1] {
		t.Fatal("h did not collapse B1")
	}
	if _, ok := m.currentGalleryItem(); ok {
		t.Fatal("cursor remained on hidden item")
	}
	if galleryRowIndex(t, m, "my box") <= m.GalleryIndex {
		t.Fatal("custom design disappeared when B1 collapsed")
	}
	m.handleGalleryKey(testKey("l"))
	if m.GalleryCollapsed[galleryB1] {
		t.Fatal("l did not expand B1")
	}
	m.GalleryIndex = galleryRowIndex(t, m, "my box")
	m.handleGalleryKey(testKey("d"))
	if m.Prompt.Mode != PromptDeleteDesign || m.Prompt.Value != "my box" {
		t.Fatalf("gallery deletion prompt = %#v", m.Prompt)
	}
}

func galleryRowIndex(t *testing.T, m Model, name string) int {
	t.Helper()
	for i, row := range m.galleryRows() {
		if !row.Header && !row.Empty && row.Item.Name == name {
			return i
		}
	}
	t.Fatalf("gallery row %q not found", name)
	return -1
}

func TestGalleryTabsKeepDraftAndConfirmBeforeReplacingIt(t *testing.T) {
	stocks := []config.LabelPreset{{Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "round"}}
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{}, stocks, "b1-50x50")
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.addTextElement()
	previous := cloneDocument(m.Document)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.Tab != tabGallery || !strings.Contains(m.View(), "Label gallery") || !strings.Contains(m.View(), "2 Gallery") {
		t.Fatalf("gallery tab not visible: %s", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.Prompt.Mode != PromptOpenGallery || len(m.Document.Elements) != len(previous.Elements) {
		t.Fatalf("unsaved work replaced without confirmation: %#v", m)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.Tab != tabGallery || len(m.Document.Elements) != 1 {
		t.Fatal("cancel did not preserve draft")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(testKey("y"))
	m = updated.(Model)
	if m.Tab != tabDesigner || m.Document.Elements[0].Type != label.ElementQR || !m.unsavedTemplate {
		t.Fatalf("starter design was not opened: %#v", m.Document)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.Tab != tabGallery || len(m.Document.Elements) != 1 {
		t.Fatal("switching tabs changed working label")
	}
}

func TestGalleryPreviewIgnoresStaleRender(t *testing.T) {
	stocks := []config.LabelPreset{{Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "round"}}
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{}, stocks, "b1-50x50")
	m.Tab = tabGallery
	first := m.requestGalleryPreview()
	m.GalleryIndex = galleryRowIndex(t, m, "b1-50x50 / QR + title")
	second := m.requestGalleryPreview()
	m.onGalleryRendered(first().(galleryRenderedMsg))
	if len(m.GalleryPNG) != 0 {
		t.Fatal("stale gallery preview replaced selection")
	}
	m.onGalleryRendered(second().(galleryRenderedMsg))
	if len(m.GalleryPNG) == 0 {
		t.Fatal("current gallery preview missing")
	}
}

func TestGalleryImagePreviewTracksSelectedLabelSize(t *testing.T) {
	stocks := []config.LabelPreset{
		{Name: "b1-wide", WidthMM: 50, HeightMM: 30, Shape: "rect"},
		{Name: "b1-square", WidthMM: 50, HeightMM: 50, Shape: "round"},
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{}, stocks, "b1-wide")
	m.Tab = tabGallery
	m.Preview.Protocol = LivePreviewKitty
	m.Width, m.Height = minTerminalWidth, minTerminalHeight
	m.reflow()
	path := filepath.Join(t.TempDir(), "terminal-image")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	previous := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = previous }()
	for _, selected := range []struct {
		name                   string
		heightPx, previewLines int
	}{{"b1-wide / QR", 240, 12}, {"b1-square / QR", 400, 16}} {
		index := galleryRowIndex(t, m, selected.name)
		m.GalleryIndex = index
		msg := m.requestGalleryPreview()().(galleryRenderedMsg)
		if msg.Err != nil {
			t.Fatalf("render gallery item %d: %v", index, msg.Err)
		}
		image, err := png.Decode(bytes.NewReader(msg.PNG))
		if err != nil {
			t.Fatalf("decode gallery item %d: %v", index, err)
		}
		if image.Bounds().Dy() != selected.heightPx {
			t.Fatalf("selected gallery PNG height = %d, want %d", image.Bounds().Dy(), selected.heightPx)
		}
		if cmd := m.onGalleryRendered(msg); cmd == nil {
			t.Fatalf("gallery item %d has no image command", index)
		} else {
			cmd()
		}
		if got := len(m.galleryPreviewLines(layoutPropertiesWidth)); got != selected.previewLines {
			t.Fatalf("gallery item %d reserved %d preview lines, want %d", index, got, selected.previewLines)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wide, square := strings.Contains(string(data), "c=20,r=5"), strings.Contains(string(data), "c=20,r=9")
	if !wide || !square {
		t.Fatalf("gallery image kept the editor's 50x30 aspect: wide=%t square=%t", wide, square)
	}
}

func TestSavedDesignCommandQuotesBindingsAndRequiresValues(t *testing.T) {
	stock := config.LabelPreset{Name: "b1-round", WidthMM: 50, HeightMM: 50, Shape: "round"}
	printer := config.PrinterProfile{Name: "b1", Model: "B1"}
	doc := label.NewDocument(50, 50)
	doc.Shape = "round"
	doc.Elements = []label.Element{{ID: "qr-1", Type: label.ElementQR, QR: &label.QRElement{Value: "https://example.org/a?x=1&y=2"}}, {ID: "text-2", Type: label.ElementText, Text: &label.TextElement{Value: "Sam's bin"}}}
	item := galleryItem{Name: "Shelf's label", Saved: true, Document: doc, Bindings: []config.DesignBinding{{Name: "title", ElementID: "text-2", Required: true}, {Name: "url", ElementID: "qr-1", Required: true}}}
	command, err := savedPrintCommand(item, printer, stock, "/path with space/config.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"niimtui print", "--config '/path with space/config.json'", "--printer 'b1'", "--preset 'b1-round'", "--design 'Shelf'\"'\"'s label'", "--set 'title=Sam'\"'\"'s bin'", "--set 'url=https://example.org/a?x=1&y=2'"} {
		if !strings.Contains(command, part) {
			t.Fatalf("command missing %q: %s", part, command)
		}
	}
	output, err := exec.Command("sh", "-c", "set -- "+strings.TrimPrefix(command, "niimtui ")+"; printf '%s\\n' \"$@\"").Output()
	if err != nil || !strings.Contains(string(output), "title=Sam's bin\n") || !strings.Contains(string(output), "Shelf's label\n") {
		t.Fatalf("quoted command arguments: %v: %s", err, output)
	}
	doc.Elements[0].QR.Value = ""
	item.Document = doc
	if _, err := savedPrintCommand(item, printer, stock, ""); err == nil || !strings.Contains(err.Error(), "url") {
		t.Fatalf("missing required binding accepted: %v", err)
	}
	if _, err := savedPrintCommand(item, printer, config.LabelPreset{Name: "wrong"}, ""); err == nil {
		t.Fatal("mismatched stock accepted")
	}
}

func TestMouseTabSelection(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Ready, m.Width, m.Height = true, minTerminalWidth, minTerminalHeight
	m.reflow()
	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: m.Width/2 + 5, Y: 1})
	m = updated.(Model)
	updated, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: m.Width/2 + 5, Y: 1})
	m = updated.(Model)
	if m.Tab != tabGallery {
		t.Fatal("clicking Gallery tab did not switch tabs")
	}
	updated, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: m.Width/2 - 5, Y: 1})
	m = updated.(Model)
	updated, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: m.Width/2 - 5, Y: 1})
	m = updated.(Model)
	if m.Tab != tabDesigner {
		t.Fatal("clicking Designer tab did not switch tabs")
	}
}

func TestMouseClickStillCollapsesGalleryHeader(t *testing.T) {
	stock := config.LabelPreset{Name: "b1-round", WidthMM: 50, HeightMM: 50, Shape: "round"}
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{}, []config.LabelPreset{stock}, stock.Name)
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.Tab = tabGallery
	m.reflow()
	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 5, Y: layoutBodyTop + 3})
	m = updated.(Model)
	updated, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 5, Y: layoutBodyTop + 3})
	m = updated.(Model)
	if !m.GalleryCollapsed[galleryB1] {
		t.Fatal("clicking B1 header did not collapse its designs")
	}
}

func TestGalleryShowsCommandForSavedDesign(t *testing.T) {
	stock := config.LabelPreset{Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "round"}
	printer := config.PrinterProfile{Name: "b1", Model: "B1", DefaultPreset: stock.Name}
	doc := label.NewDocument(50, 50)
	doc.Shape = "round"
	m := NewModelWithPresets(50, 50, "round", "", PrintConfig{
		Printers: []config.PrinterProfile{printer}, Printer: printer.Name, Model: printer.Model,
		ConfigPath: "/tmp/label config.json", DesignPresets: []config.DesignPreset{{Name: "my label", Document: doc}},
	}, []config.LabelPreset{stock}, stock.Name)
	m.GalleryIndex = galleryRowIndex(t, m, "my label")
	m.showGalleryCommand()
	if m.Prompt.Mode != PromptCommand || !strings.Contains(m.Prompt.Value, "--design 'my label'") {
		t.Fatalf("command dialog = %#v", m.Prompt)
	}
	m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Prompt.Mode != PromptNone {
		t.Fatal("command dialog did not close")
	}
}

func TestSavedPrintCommandOmitsDefaultConfigButKeepsCustomConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defaultPath, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	stock := config.LabelPreset{Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "round"}
	doc := label.NewDocument(50, 50)
	doc.Shape = "round"
	model := NewModelWithPresets(50, 50, "round", "", PrintConfig{
		Printer: "b1", Model: "B1", ConfigPath: defaultPath,
		Printers:      []config.PrinterProfile{{Name: "b1", Model: "B1"}},
		DesignPresets: []config.DesignPreset{{Name: "my label", Document: doc}},
	}, []config.LabelPreset{stock}, stock.Name)
	model.GalleryIndex = galleryRowIndex(t, model, "my label")
	model.showGalleryCommand()
	if strings.Contains(model.Prompt.Value, "--config") {
		t.Fatalf("default path in print command: %q", model.Prompt.Value)
	}
	model.Prompt = PromptState{}
	model.Print.ConfigPath = filepath.Join(t.TempDir(), "other config.json")
	model.showGalleryCommand()
	if !strings.Contains(model.Prompt.Value, "--config '") {
		t.Fatalf("custom config path omitted: %q", model.Prompt.Value)
	}
	t.Setenv("XDG_CONFIG_HOME", ".relative-config")
	relativeDefault, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	model.Prompt = PromptState{}
	model.Print.ConfigPath = relativeDefault
	model.showGalleryCommand()
	if strings.Contains(model.Prompt.Value, "--config") {
		t.Fatalf("relative default path in print command: %q", model.Prompt.Value)
	}
}
