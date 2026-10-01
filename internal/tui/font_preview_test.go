package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/image/font/gofont/gobold"

	"niimtui/internal/render"
)

func fontPreviewModel(t *testing.T) (Model, string) {
	t.Helper()
	path := writeTestFont(t, "Go-Regular.ttf")
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width, m.Height, m.Ready = minTerminalWidth, minTerminalHeight, true
	m.reflow()
	m.addTextElement()
	m.applyTextBuffer("Garage Bin")
	m.commitHistory("set text")
	m.Fonts = []FontOption{{Name: "Default"}, {Name: "Go-Regular", Path: path}}
	m.Preview.Protocol = LivePreviewKitty
	m.Preview.LastKey = m.livePreviewKey()
	return m, path
}

func TestFontPickerScrollingPreviewsCanvasAndImageWithoutSaving(t *testing.T) {
	m, fontPath := fontPreviewModel(t)
	initial := cloneDocument(m.Document)
	history := m.History
	initialRender, err := render.RenderDocument(m.Document)
	if err != nil {
		t.Fatal(err)
	}
	initialCanvas := renderCanvas(m)
	m.toggleFontPicker()
	before := m.Preview.RequestedSeq
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.FontPickerIndex != 1 || m.Preview.RequestedSeq <= before {
		t.Fatalf("scroll did not trigger image preview: index=%d seq=%d", m.FontPickerIndex, m.Preview.RequestedSeq)
	}
	preview := m.previewDocument()
	element, ok := preview.ElementByID(m.SelectedID)
	if !ok || element.Text.FontPath != fontPath {
		t.Fatalf("previewed font path = %q, want %q", element.Text.FontPath, fontPath)
	}
	if !reflect.DeepEqual(m.Document, initial) || m.History != history || m.FontPath != "" {
		t.Fatal("scrolling saved the font before Enter")
	}
	msg := renderLivePreviewCmd(m, m.Preview.RequestedSeq)().(livePreviewRenderedMsg)
	if bytes.Equal(initialRender.PreviewPNG, msg.PNG) {
		t.Fatal("image preview still shows the old font")
	}
	if renderCanvas(m) == initialCanvas {
		t.Fatal("canvas preview still shows the old font")
	}
	if properties := strings.Join(propertyPanelLines(m, layoutPropertiesWidth), "\n"); !strings.Contains(properties, "Go-Regular") {
		t.Fatalf("canvas properties did not show previewed font: %q", properties)
	}
}

func TestFontPickerSearchAndEscapeRestoreOriginalPreview(t *testing.T) {
	m, fontPath := fontPreviewModel(t)
	initial := cloneDocument(m.Document)
	m.toggleFontPicker()
	updated, _ := m.Update(testKey("g"))
	m = updated.(Model)
	if m.FontPickerIndex != 1 || m.previewDocument().Elements[0].Text.FontPath != fontPath {
		t.Fatal("filtered search did not preview matching font")
	}
	updated, _ = m.Update(testKey("z"))
	m = updated.(Model)
	if !reflect.DeepEqual(m.previewDocument(), initial) {
		t.Fatal("empty search results kept previewing an unrelated font")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	if m.previewDocument().Elements[0].Text.FontPath != fontPath {
		t.Fatal("returning to matching results did not restore live font preview")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if !m.FontPickerOpen || m.FontPickerSearch || m.previewDocument().Elements[0].Text.FontPath != fontPath {
		t.Fatal("leaving search discarded highlighted font too early")
	}
	beforeClose := m.Preview.RequestedSeq
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.FontPickerOpen || m.Preview.RequestedSeq <= beforeClose || !reflect.DeepEqual(m.previewDocument(), initial) || !reflect.DeepEqual(m.Document, initial) {
		t.Fatal("closing font picker did not restore original font")
	}
}

func TestFontPickerBrowsingDifferentFontsChangesPreviewBeforeEnter(t *testing.T) {
	m, regularPath := fontPreviewModel(t)
	boldPath := filepath.Join(t.TempDir(), "Go-Bold.ttf")
	if err := os.WriteFile(boldPath, gobold.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	m.Fonts = []FontOption{{Name: "Default"}, {Name: "Go-Regular", Path: regularPath}, {Name: "Go-Bold", Path: boldPath}}
	m.toggleFontPicker()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	first := renderLivePreviewCmd(m, m.Preview.RequestedSeq)().(livePreviewRenderedMsg)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	second := renderLivePreviewCmd(m, m.Preview.RequestedSeq)().(livePreviewRenderedMsg)
	if m.FontPickerIndex != 2 || bytes.Equal(first.PNG, second.PNG) {
		t.Fatal("moving to a second font did not update the image preview")
	}
	if selected, _ := m.selectedElement(); selected.Text.FontPath != "" {
		t.Fatal("scrolling through fonts changed saved text")
	}
}

func TestFontPickerEnterCommitsOneFontChange(t *testing.T) {
	m, fontPath := fontPreviewModel(t)
	history := m.History
	m.toggleFontPicker()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	preview := m.previewDocument()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.FontPickerOpen || m.FontPath != fontPath || !reflect.DeepEqual(m.Document, preview) || m.History.Parent != history {
		t.Fatal("Enter did not commit the previewed font once")
	}
}

func TestOutsideClickCancelsUncommittedFontPreview(t *testing.T) {
	m, _ := fontPreviewModel(t)
	initial := cloneDocument(m.Document)
	m.toggleFontPicker()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(mouseSelectionMsg(tea.MouseActionPress, 0, 0))
	m = updated.(Model)
	if m.FontPickerOpen || !reflect.DeepEqual(m.previewDocument(), initial) {
		t.Fatal("outside click left a temporary font applied")
	}
}
