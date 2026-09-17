package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/image/font/gofont/goregular"

	"niimtui/internal/label"
)

func TestAutoFitTextElementGrowsHeightForWrappedText(t *testing.T) {
	element := label.NewTextElement("title", "Storage Box 12", 0, 0, 10, 4, 18)
	autoFitTextElement(&element)
	if element.HeightMM <= 4 {
		t.Fatalf("height = %.2f, want greater than 4", element.HeightMM)
	}
}

func TestResizeElementKeepsWrappedTextInsideHeight(t *testing.T) {
	original := label.NewTextElement("title", "Storage Box 12", 0, 0, 12, 8, 18)
	updated := resizeElement(original, HandleBottom, 0, -20)
	if updated.HeightMM >= original.HeightMM && updated.HeightMM <= minElementHeightMM {
		t.Fatalf("height = %.2f, expected wrapped-text minimum larger than generic minimum", updated.HeightMM)
	}
	_, minHeight := minimumElementSize(updated)
	if updated.HeightMM < minHeight {
		t.Fatalf("height = %.2f, min height = %.2f", updated.HeightMM, minHeight)
	}
}

func TestResizeQRElementKeepsSquareAspectFromVerticalHandle(t *testing.T) {
	original := label.NewQRElement("qr", "https://example.com", 10, 5, 12)
	updated := resizeElement(original, HandleBottom, 0, 8)
	if updated.WidthMM != updated.HeightMM {
		t.Fatalf("QR size = %.2fx%.2f, want square", updated.WidthMM, updated.HeightMM)
	}
	if updated.HeightMM <= original.HeightMM {
		t.Fatalf("QR height = %.2f, want greater than %.2f", updated.HeightMM, original.HeightMM)
	}
}

func TestDoubleClickTextElementBeginsEditing(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Canvas = newCanvas(80, 24, m.Document.WidthMM, m.Document.HeightMM)
	element, ok := m.selectedElement()
	if !ok || element.Text == nil {
		t.Fatal("expected selected text element")
	}
	x, y := elementClickPoint(m, element)
	now := time.Now()

	m.handleMousePressAt(leftClick(x, y), now)
	m.handleMousePressAt(leftClick(x, y), now.Add(100*time.Millisecond))

	if !m.EditingText {
		t.Fatal("double click did not begin text editing")
	}
	if m.TextBuffer != element.Text.Value {
		t.Fatalf("text buffer = %q, want %q", m.TextBuffer, element.Text.Value)
	}
}

func TestDoubleClickQRElementBeginsEditing(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Canvas = newCanvas(80, 24, m.Document.WidthMM, m.Document.HeightMM)
	m.addQRElement()
	m.EditingText = false
	m.TextBuffer = ""
	element, ok := m.selectedElement()
	if !ok || element.QR == nil {
		t.Fatal("expected selected QR element")
	}
	x, y := elementClickPoint(m, element)
	now := time.Now()

	m.handleMousePressAt(leftClick(x, y), now)
	m.handleMousePressAt(leftClick(x, y), now.Add(100*time.Millisecond))

	if !m.EditingText {
		t.Fatal("double click did not begin QR editing")
	}
	if m.TextBuffer != element.QR.Value {
		t.Fatalf("text buffer = %q, want %q", m.TextBuffer, element.QR.Value)
	}
}

func TestArrowKeysMoveSelectedElement(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	initial, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRight}) {
		t.Fatal("right arrow was not handled")
	}
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyDown}) {
		t.Fatal("down arrow was not handled")
	}
	updated, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element after movement")
	}
	if updated.XMM != initial.XMM+1 || updated.YMM != initial.YMM+1 {
		t.Fatalf("position = %.1f, %.1f; want %.1f, %.1f", updated.XMM, updated.YMM, initial.XMM+1, initial.YMM+1)
	}
}

func TestMovingQRElementKeepsScreenRectSize(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Canvas = newCanvas(80, 24, m.Document.WidthMM, m.Document.HeightMM)
	m.addQRElement()
	m.EditingText = false
	element, ok := m.selectedElement()
	if !ok || element.QR == nil {
		t.Fatal("expected selected QR element")
	}
	before := m.elementScreenRect(element)

	if !m.nudgeSelected(0, 1) {
		t.Fatal("nudgeSelected() = false, want true")
	}
	element, ok = m.selectedElement()
	if !ok || element.QR == nil {
		t.Fatal("expected selected QR element after move")
	}
	after := m.elementScreenRect(element)

	if before.right-before.left != after.right-after.left || before.bottom-before.top != after.bottom-after.top {
		t.Fatalf("screen size changed from %dx%d to %dx%d", before.right-before.left, before.bottom-before.top, after.right-after.left, after.bottom-after.top)
	}
}

func TestBracketKeysResizeSelectedElementFromBottomRight(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	initial, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")}) {
		t.Fatal("] was not handled")
	}
	grown, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element after grow")
	}
	if grown.WidthMM <= initial.WidthMM || grown.HeightMM <= initial.HeightMM {
		t.Fatalf("grown size = %.1f x %.1f; want larger than %.1f x %.1f", grown.WidthMM, grown.HeightMM, initial.WidthMM, initial.HeightMM)
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")}) {
		t.Fatal("[ was not handled")
	}
	shrunk, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element after shrink")
	}
	if shrunk.WidthMM >= grown.WidthMM || shrunk.HeightMM >= grown.HeightMM {
		t.Fatalf("shrunk size = %.1f x %.1f; want smaller than %.1f x %.1f", shrunk.WidthMM, shrunk.HeightMM, grown.WidthMM, grown.HeightMM)
	}
}

func TestNewModelAppliesFontPathToInitialAndAddedText(t *testing.T) {
	m := NewModel(50, 30, "rect", "/tmp/example.ttf", PrintConfig{})
	initial, ok := m.selectedElement()
	if !ok || initial.Text == nil {
		t.Fatal("expected initial selected text element")
	}
	if initial.Text.FontPath != "/tmp/example.ttf" {
		t.Fatalf("initial font path = %q, want custom path", initial.Text.FontPath)
	}

	m.addTextElement()
	added, ok := m.selectedElement()
	if !ok || added.Text == nil {
		t.Fatal("expected added selected text element")
	}
	if added.Text.FontPath != "/tmp/example.ttf" {
		t.Fatalf("added font path = %q, want custom path", added.Text.FontPath)
	}
}

func TestApplySelectedFontUpdatesSelectedTextAndDefault(t *testing.T) {
	fontPath := writeTestFont(t, "Go-Regular.ttf")
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Fonts = []FontOption{
		{Name: "Default", Path: ""},
		{Name: "Go-Regular", Path: fontPath},
	}
	m.FontPickerOpen = true
	m.FontPickerIndex = 1

	if !m.applySelectedFont() {
		t.Fatal("applySelectedFont() = false, want true")
	}
	selected, ok := m.selectedElement()
	if !ok || selected.Text == nil {
		t.Fatal("expected selected text element")
	}
	if selected.Text.FontPath != fontPath {
		t.Fatalf("selected font path = %q, want %q", selected.Text.FontPath, fontPath)
	}
	if m.FontPath != fontPath {
		t.Fatalf("model font path = %q, want %q", m.FontPath, fontPath)
	}
	if m.FontPickerOpen {
		t.Fatal("font picker still open after apply")
	}

	m.addTextElement()
	added, ok := m.selectedElement()
	if !ok || added.Text == nil {
		t.Fatal("expected added text element")
	}
	if added.Text.FontPath != fontPath {
		t.Fatalf("added font path = %q, want %q", added.Text.FontPath, fontPath)
	}
}

func TestNewModelAddsCurrentFontPathAsFilenameOption(t *testing.T) {
	fontPath := filepath.Join(t.TempDir(), "ExampleNerdFont-Regular.ttf")
	m := NewModel(50, 30, "rect", fontPath, PrintConfig{})

	for _, font := range m.Fonts {
		if font.Path == fontPath {
			if font.Name != "ExampleNerdFont-Regular" {
				t.Fatalf("font name = %q, want filename without extension", font.Name)
			}
			return
		}
	}
	t.Fatalf("font path %q not found in options", fontPath)
}

func TestFontPickerSearchFiltersAndAppliesMatch(t *testing.T) {
	jetBrainsPath := writeTestFont(t, "JetBrainsMonoNerdFont-Regular.ttf")
	goRegularPath := writeTestFont(t, "Go-Regular.ttf")
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Fonts = []FontOption{
		{Name: "Default", Path: ""},
		{Name: "Go-Regular", Path: goRegularPath},
		{Name: "JetBrainsMonoNerdFont-Regular", Path: jetBrainsPath},
	}
	m.FontPickerOpen = true
	m.FontPickerSearch = true

	if !m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jet")}) {
		t.Fatal("handleFontPickerKey() = false, want true")
	}
	if m.FontPickerQuery != "jet" {
		t.Fatalf("font picker query = %q, want jet", m.FontPickerQuery)
	}
	if m.FontPickerIndex != 2 {
		t.Fatalf("font picker index = %d, want JetBrains index", m.FontPickerIndex)
	}

	indices := m.filteredFontIndices()
	if len(indices) != 1 || indices[0] != 2 {
		t.Fatalf("filtered indices = %v, want [2]", indices)
	}
	if !m.applySelectedFont() {
		t.Fatal("applySelectedFont() = false, want true")
	}
	selected, ok := m.selectedElement()
	if !ok || selected.Text == nil {
		t.Fatal("expected selected text element")
	}
	if selected.Text.FontPath != jetBrainsPath {
		t.Fatalf("selected font path = %q, want %q", selected.Text.FontPath, jetBrainsPath)
	}
}

func TestFontPickerBackspaceUpdatesSearch(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Fonts = []FontOption{
		{Name: "Default", Path: ""},
		{Name: "Alpha", Path: "/tmp/alpha.ttf"},
		{Name: "Alpine", Path: "/tmp/alpine.ttf"},
	}
	m.FontPickerOpen = true
	m.FontPickerQuery = "alph"
	m.FontPickerIndex = 1

	if !m.removeFontSearchRune() {
		t.Fatal("removeFontSearchRune() = false, want true")
	}
	if m.FontPickerQuery != "alp" {
		t.Fatalf("font picker query = %q, want alp", m.FontPickerQuery)
	}
	indices := m.filteredFontIndices()
	if len(indices) != 2 || indices[0] != 1 || indices[1] != 2 {
		t.Fatalf("filtered indices = %v, want [1 2]", indices)
	}
}

func TestFontPickerSearchModeUsesCtrlNAndCtrlP(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Fonts = []FontOption{
		{Name: "Default", Path: ""},
		{Name: "Alpha", Path: "/tmp/alpha.ttf"},
		{Name: "Alpine", Path: "/tmp/alpine.ttf"},
	}
	m.FontPickerOpen = true
	m.FontPickerSearch = true
	m.FontPickerQuery = "alp"
	m.FontPickerIndex = 1

	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.FontPickerIndex != 2 {
		t.Fatalf("font picker index after ctrl+n = %d, want 2", m.FontPickerIndex)
	}
	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.FontPickerIndex != 1 {
		t.Fatalf("font picker index after ctrl+p = %d, want 1", m.FontPickerIndex)
	}
}

func TestFontPickerEscapeSwitchesToBrowseMode(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Fonts = []FontOption{
		{Name: "Default", Path: ""},
		{Name: "Alpha", Path: "/tmp/alpha.ttf"},
		{Name: "Alpine", Path: "/tmp/alpine.ttf"},
	}
	m.FontPickerOpen = true
	m.FontPickerSearch = true
	m.FontPickerQuery = "alp"
	m.FontPickerIndex = 1

	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.FontPickerOpen {
		t.Fatal("font picker closed on first esc, want browse mode")
	}
	if m.FontPickerSearch {
		t.Fatal("font picker still in search mode after esc")
	}
	if m.FontPickerQuery != "alp" {
		t.Fatalf("font picker query = %q, want alp", m.FontPickerQuery)
	}
	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.FontPickerSearch {
		t.Fatal("font picker did not return to search mode after /")
	}
	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.FontPickerSearch {
		t.Fatal("font picker still in search mode after second search esc")
	}

	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.FontPickerIndex != 2 {
		t.Fatalf("font picker index after browse j = %d, want 2", m.FontPickerIndex)
	}
	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.FontPickerOpen {
		t.Fatal("font picker still open after second esc")
	}
}

func TestFontPickerBrowseModePagesWithCtrlDAndCtrlU(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Fonts = []FontOption{{Name: "Default", Path: ""}}
	for i := 1; i <= 12; i++ {
		m.Fonts = append(m.Fonts, FontOption{Name: fmt.Sprintf("Font-%02d", i), Path: fmt.Sprintf("/tmp/font-%02d.ttf", i)})
	}
	m.FontPickerOpen = true
	m.FontPickerSearch = false
	m.FontPickerIndex = 1

	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.FontPickerIndex != 8 {
		t.Fatalf("font picker index after ctrl+d = %d, want 8", m.FontPickerIndex)
	}
	m.handleFontPickerKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.FontPickerIndex != 1 {
		t.Fatalf("font picker index after ctrl+u = %d, want 1", m.FontPickerIndex)
	}
}

func writeTestFont(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, goregular.TTF, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func leftClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

func elementClickPoint(m Model, element label.Element) (int, int) {
	r := m.elementScreenRect(element)
	return r.left + max((r.right-r.left)/2, 1), r.top + max((r.bottom-r.top)/2, 1)
}
