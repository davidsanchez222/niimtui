package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/image/font/gofont/goregular"

	"niimtui/internal/config"
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

func TestPlusMinusResizeSelectedQRElement(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addQRElement()
	initial, ok := m.selectedElement()
	if !ok || initial.QR == nil {
		t.Fatal("expected selected QR element")
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("+")}) {
		t.Fatal("+ was not handled for QR")
	}
	grown, ok := m.selectedElement()
	if !ok || grown.QR == nil {
		t.Fatal("expected selected QR element after grow")
	}
	if grown.WidthMM != initial.WidthMM+1 || grown.HeightMM != initial.HeightMM+1 {
		t.Fatalf("grown QR size = %.1fx%.1f, want %.1fx%.1f", grown.WidthMM, grown.HeightMM, initial.WidthMM+1, initial.HeightMM+1)
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-")}) {
		t.Fatal("- was not handled for QR")
	}
	shrunk, ok := m.selectedElement()
	if !ok || shrunk.QR == nil {
		t.Fatal("expected selected QR element after shrink")
	}
	if shrunk.WidthMM != initial.WidthMM || shrunk.HeightMM != initial.HeightMM {
		t.Fatalf("shrunk QR size = %.1fx%.1f, want %.1fx%.1f", shrunk.WidthMM, shrunk.HeightMM, initial.WidthMM, initial.HeightMM)
	}
}

func TestDoubleClickTextElementBeginsEditing(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	m.Canvas = newCanvas(160, 60, m.Document.WidthMM, m.Document.HeightMM)
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
	m.addTextElement()
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

func TestLowercaseMoveKeepsScreenRectSizeAcrossRoundingBoundaries(t *testing.T) {
	m := NewModel(50, 50, "round", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()
	element := label.NewQRElement("qr", "https://example.com", 10, 0, 18)
	m.Document.Elements = []label.Element{element}
	m.SelectedID = element.ID

	for y := 0.0; y <= m.Document.HeightMM-element.HeightMM-1; y++ {
		element.YMM = y
		if !m.Document.UpdateElement(element) {
			t.Fatal("failed to position QR element")
		}
		before := m.elementScreenRect(element)
		if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) {
			t.Fatal("j was not handled")
		}
		moved, ok := m.selectedElement()
		if !ok {
			t.Fatal("selected element missing")
		}
		after := m.elementScreenRect(moved)
		if before.right-before.left != after.right-after.left || before.bottom-before.top != after.bottom-after.top {
			t.Fatalf("y %.1f screen size changed from %dx%d to %dx%d", y, before.right-before.left, before.bottom-before.top, after.right-after.left, after.bottom-after.top)
		}
		element = moved
	}
}

func TestLowercaseMoveMovesScreenRectOneCell(t *testing.T) {
	m := NewModel(50, 50, "round", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()
	element := label.NewQRElement("qr", "https://example.com", 5, 14, 18)
	m.Document.Elements = []label.Element{element}
	m.SelectedID = element.ID
	before := m.elementScreenRect(element)

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) {
		t.Fatal("j was not handled")
	}
	moved, ok := m.selectedElement()
	if !ok {
		t.Fatal("selected element missing")
	}
	after := m.elementScreenRect(moved)

	if after.top != before.top+1 || after.bottom != before.bottom+1 {
		t.Fatalf("screen y moved from %d..%d to %d..%d, want one-cell move", before.top, before.bottom, after.top, after.bottom)
	}
	if after.right-after.left != before.right-before.left || after.bottom-after.top != before.bottom-before.top {
		t.Fatalf("screen size changed from %dx%d to %dx%d", before.right-before.left, before.bottom-before.top, after.right-after.left, after.bottom-after.top)
	}
}

func TestArrowMoveFromFractionalPositionMovesScreenRectOneCell(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()
	element := label.NewQRElement("qr", "https://example.com", 15.5, 13.5, 12)
	m.Document.Elements = []label.Element{element}
	m.SelectedID = element.ID
	before := m.elementScreenRect(element)

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyUp}) {
		t.Fatal("up arrow was not handled")
	}
	moved, ok := m.selectedElement()
	if !ok {
		t.Fatal("selected element missing")
	}
	after := m.elementScreenRect(moved)

	if after.top != before.top-1 || after.bottom != before.bottom-1 {
		t.Fatalf("screen y moved from %d..%d to %d..%d, want one-cell up move", before.top, before.bottom, after.top, after.bottom)
	}
	if after.right-after.left != before.right-before.left || after.bottom-after.top != before.bottom-before.top {
		t.Fatalf("screen size changed from %dx%d to %dx%d", before.right-before.left, before.bottom-before.top, after.right-after.left, after.bottom-after.top)
	}
}

func TestBracketKeysResizeSelectedElementFromBottomRight(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
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

func TestCapitalHJKLResizeSelectedElementDimensions(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	initial, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}
	initial.WidthMM = 25
	initial.HeightMM = 20
	if !m.Document.UpdateElement(initial) {
		t.Fatal("failed to expand selected element for test")
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")}) {
		t.Fatal("H was not handled")
	}
	shrunkWidth, _ := m.selectedElement()
	if shrunkWidth.XMM != initial.XMM || shrunkWidth.WidthMM >= initial.WidthMM {
		t.Fatalf("after H x/width = %.1f/%.1f, want same x and narrower than %.1f/%.1f", shrunkWidth.XMM, shrunkWidth.WidthMM, initial.XMM, initial.WidthMM)
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")}) {
		t.Fatal("L was not handled")
	}
	grownWidth, _ := m.selectedElement()
	if grownWidth.WidthMM <= shrunkWidth.WidthMM {
		t.Fatalf("after L width = %.1f, want greater than %.1f", grownWidth.WidthMM, shrunkWidth.WidthMM)
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")}) {
		t.Fatal("K was not handled")
	}
	shrunkHeight, _ := m.selectedElement()
	if shrunkHeight.YMM != grownWidth.YMM || shrunkHeight.HeightMM >= grownWidth.HeightMM {
		t.Fatalf("after K y/height = %.1f/%.1f, want same y and shorter than %.1f/%.1f", shrunkHeight.YMM, shrunkHeight.HeightMM, grownWidth.YMM, grownWidth.HeightMM)
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")}) {
		t.Fatal("J was not handled")
	}
	grownHeight, _ := m.selectedElement()
	if grownHeight.HeightMM <= shrunkHeight.HeightMM {
		t.Fatalf("after J height = %.1f, want greater than %.1f", grownHeight.HeightMM, shrunkHeight.HeightMM)
	}
}

func TestAutoInsertDefaultOffForNewElements(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	if m.EditingText {
		t.Fatal("text element entered edit mode with auto insert off")
	}
	m.addQRElement()
	if m.EditingText {
		t.Fatal("QR element entered edit mode with auto insert off")
	}
}

func TestAutoInsertOptionEntersEditModeForNewElements(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.AutoInsert = true
	m.addTextElement()
	if !m.EditingText {
		t.Fatal("text element did not enter edit mode with auto insert on")
	}
}

func TestShiftArrowsAreNotFastMoveCommands(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	if m.handleCommandKey(tea.KeyMsg{Type: tea.KeyShiftRight}) {
		t.Fatal("shift+right should not be handled")
	}
}

func TestFocusPickerSelectsElementByHint(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addQRElement()
	m.SelectedID = ""

	if !m.openFocusPicker() {
		t.Fatal("openFocusPicker() = false, want true")
	}
	m.handleFocusPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.FocusPickerOpen {
		t.Fatal("focus picker still open after selecting hint")
	}
	if m.SelectedID != "qr-1" {
		t.Fatalf("selected id = %q, want qr-1", m.SelectedID)
	}
}

func TestNewModelAppliesFontPathToAddedText(t *testing.T) {
	m := NewModel(50, 30, "rect", "/tmp/example.ttf", PrintConfig{})
	if len(m.Document.Elements) != 0 {
		t.Fatalf("initial element count = %d, want 0", len(m.Document.Elements))
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

func TestPrinterArtLoadsModelArt(t *testing.T) {
	for _, model := range []string{"B1", "D110"} {
		art := printerArt(model)
		if len(art) == 0 {
			t.Fatalf("printerArt(%q) returned no art", model)
		}
	}
}

func TestPrinterArtFallsBackForUnknownModel(t *testing.T) {
	art := printerArt("")
	if len(art) == 0 {
		t.Fatal("printerArt() returned no fallback art")
	}
}

func TestApplySelectedFontUpdatesSelectedTextAndDefault(t *testing.T) {
	fontPath := writeTestFont(t, "Go-Regular.ttf")
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
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
	m.addTextElement()
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
	if lines := strings.Join(fontPickerLines(m, 28), "\n"); !strings.ContainsRune(lines, promptCursorRune) {
		t.Fatalf("font picker lines = %q, want cursor %q", lines, promptCursorRune)
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

func TestCapitalFOpensFontPickerForText(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")}) {
		t.Fatal("F was not handled")
	}
	if !m.FontPickerOpen || !m.FontPickerSearch {
		t.Fatal("font picker did not open in search mode")
	}

	m.closeFontPicker()
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")}) {
		t.Fatal("f was not handled")
	}
	if !m.FocusPickerOpen {
		t.Fatal("lowercase f did not open focus picker")
	}
}

func TestCtrlCQuitsWhileFontPickerIsSearching(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.FontPickerOpen = true
	m.FontPickerSearch = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c returned nil command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command msg = %T, want tea.QuitMsg", msg)
	}
}

func TestQAddsQRElementInCanvasMode(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q quit in canvas mode, want add QR")
		}
	}
	model := updated.(Model)
	if len(model.Document.Elements) != 1 || model.Document.Elements[0].QR == nil {
		t.Fatalf("elements = %#v, want one QR element", model.Document.Elements)
	}
}

func TestRRotatesSelectedElementWithoutAddingQR(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	initialCount := len(m.Document.Elements)
	initial, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}) {
		t.Fatal("r was not handled")
	}
	rotated, ok := m.selectedElement()
	if !ok {
		t.Fatal("selected element missing")
	}
	if len(m.Document.Elements) != initialCount {
		t.Fatalf("element count = %d, want %d", len(m.Document.Elements), initialCount)
	}
	if rotated.Rotation != 90 {
		t.Fatalf("rotation = %d, want 90", rotated.Rotation)
	}
	if rotated.WidthMM != initial.HeightMM || rotated.HeightMM != initial.WidthMM {
		t.Fatalf("size = %.1fx%.1f, want %.1fx%.1f", rotated.WidthMM, rotated.HeightMM, initial.HeightMM, initial.WidthMM)
	}
}

func TestCapitalRRotatesCanvasAndElements(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	element := label.NewTextElement("title", "Box", 5, 4, 20, 8, 18)
	m.Document.Elements = []label.Element{element}
	m.SelectedID = element.ID
	m.Width = 160
	m.Height = 30
	m.reflow()

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")}) {
		t.Fatal("R was not handled")
	}
	if m.Document.WidthMM != 30 || m.Document.HeightMM != 50 || m.Document.Rotation != 90 {
		t.Fatalf("document = %.1fx%.1f rot %d, want 30x50 rot 90", m.Document.WidthMM, m.Document.HeightMM, m.Document.Rotation)
	}
	rotated, ok := m.selectedElement()
	if !ok {
		t.Fatal("selected element missing")
	}
	if rotated.XMM != 18 || rotated.YMM != 5 || rotated.WidthMM != 8 || rotated.HeightMM != 20 || rotated.Rotation != 90 {
		t.Fatalf("rotated element = x %.1f y %.1f size %.1fx%.1f rot %d, want x 18 y 5 size 8x20 rot 90", rotated.XMM, rotated.YMM, rotated.WidthMM, rotated.HeightMM, rotated.Rotation)
	}
}

func TestCopyPasteSelectedComponent(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	element, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}
	element.Text.Value = "Copied"
	m.Document.UpdateElement(element)

	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}) {
		t.Fatal("copy key was not handled")
	}
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")}) {
		t.Fatal("paste key was not handled")
	}
	if len(m.Document.Elements) != 2 {
		t.Fatalf("element count = %d, want 2", len(m.Document.Elements))
	}
	pasted, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected pasted selection")
	}
	if pasted.ID == element.ID {
		t.Fatal("pasted element reused source ID")
	}
	if pasted.Text == nil || pasted.Text.Value != "Copied" {
		t.Fatalf("pasted text = %#v, want Copied", pasted.Text)
	}
}

func TestCutSelectedComponentCanUndo(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}) {
		t.Fatal("cut key was not handled")
	}
	if len(m.Document.Elements) != 0 || len(m.Clipboard) != 1 {
		t.Fatalf("after cut elements=%d clipboard=%d, want 0 and 1", len(m.Document.Elements), len(m.Clipboard))
	}
	m.undo()
	if len(m.Document.Elements) != 1 {
		t.Fatalf("after undo elements=%d, want 1", len(m.Document.Elements))
	}
}

func TestDuplicateSelectedComponent(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addQRElement()
	source, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected QR")
	}
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}) {
		t.Fatal("duplicate key was not handled")
	}
	duplicated, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected duplicated selection")
	}
	if len(m.Document.Elements) != 2 || duplicated.ID == source.ID || duplicated.QR == nil || duplicated.QR.Value != source.QR.Value {
		t.Fatalf("duplicated element = %#v elements=%d", duplicated, len(m.Document.Elements))
	}
}

func TestUndoRedoDocumentEdits(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	m.addQRElement()
	if len(m.Document.Elements) != 2 {
		t.Fatalf("element count = %d, want 2", len(m.Document.Elements))
	}
	m.undo()
	if len(m.Document.Elements) != 1 {
		t.Fatalf("after undo elements=%d, want 1", len(m.Document.Elements))
	}
	m.redo()
	if len(m.Document.Elements) != 2 {
		t.Fatalf("after redo elements=%d, want 2", len(m.Document.Elements))
	}
}

func TestUndoTreeCyclesRedoBranches(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.addTextElement()
	m.addQRElement()
	m.undo()
	m.addTextElement()
	m.undo()
	if m.History == nil || len(m.History.Children) != 2 {
		t.Fatalf("redo branches = %d, want 2", len(m.History.Children))
	}
	m.cycleRedoBranch()
	m.redo()
	if _, ok := m.Document.ElementByID("qr-2"); !ok {
		t.Fatalf("redo branch did not restore qr-2: %#v", m.Document.Elements)
	}
}

func TestMouseDragCreatesSingleUndoStep(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Width = 160
	m.Height = 30
	m.reflow()
	m.addTextElement()
	before := m.History
	element, ok := m.selectedElement()
	if !ok {
		t.Fatal("expected selected element")
	}
	x, y := elementClickPoint(m, element)
	m.handleMousePressAt(leftClick(x, y), time.Now())
	m.handleMouseMotion(tea.MouseMsg{X: x + 2, Y: y + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion})
	model, _ := m.updateMouse(tea.MouseMsg{X: x + 2, Y: y + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	m = model.(Model)
	if m.History == before || m.History.Parent != before {
		t.Fatal("drag did not create exactly one history child")
	}
	m.undo()
	restored, ok := m.selectedElement()
	if !ok || restored.XMM != element.XMM || restored.YMM != element.YMM {
		t.Fatalf("undo drag restored = %#v, want original %#v", restored, element)
	}
}

func TestQQuitsFromMenu(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.MenuOpen = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q from menu returned nil command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q from menu command msg = %T, want tea.QuitMsg", msg)
	}
}

func TestMenuTogglesAutoInsert(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	if !m.toggleMenu() {
		t.Fatal("toggleMenu() = false, want true")
	}
	if !m.MenuOpen {
		t.Fatal("menu did not open")
	}
	m.MenuIndex = 3
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.AutoInsert {
		t.Fatal("auto insert was not enabled")
	}
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.AutoInsert {
		t.Fatal("auto insert was not disabled")
	}
}

func TestMenuSelectsLabelRollAndCloses(t *testing.T) {
	presets := []config.LabelPreset{
		{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "blank"},
		{Name: "b1-50x50", WidthMM: 50, HeightMM: 50, Shape: "rect", Layout: "blank"},
	}
	m := NewModelWithPresets(50, 30, "rect", "", PrintConfig{}, presets, "b1-50x30")
	if !m.toggleMenu() {
		t.Fatal("toggleMenu() = false, want true")
	}
	m.MenuIndex = 0
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.MenuListMode != MenuListLabelRolls || m.MenuListIndex != 0 {
		t.Fatalf("menu list = %q index %d, want label rolls index 0", m.MenuListMode, m.MenuListIndex)
	}
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})

	if m.MenuOpen || m.MenuListMode != MenuListNone {
		t.Fatalf("menu state = open %t list %q, want closed", m.MenuOpen, m.MenuListMode)
	}
	if m.Preset != 1 || m.Document.HeightMM != 50 {
		t.Fatalf("selected preset = %d height %.1f, want preset 1 height 50", m.Preset, m.Document.HeightMM)
	}
}

func TestMenuSelectsSavedPresetAndCloses(t *testing.T) {
	first := label.NewDocument(50, 30)
	second := label.NewDocument(40, 12)
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.DesignPresets = []config.DesignPreset{
		{Name: "large", Document: first},
		{Name: "small", Document: second},
	}
	if !m.toggleMenu() {
		t.Fatal("toggleMenu() = false, want true")
	}
	m.MenuIndex = 1
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.MenuListMode != MenuListDesignPresets || m.MenuListIndex != 0 {
		t.Fatalf("menu list = %q index %d, want saved presets index 0", m.MenuListMode, m.MenuListIndex)
	}
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})

	if m.MenuOpen || m.MenuListMode != MenuListNone {
		t.Fatalf("menu state = open %t list %q, want closed", m.MenuOpen, m.MenuListMode)
	}
	if m.DesignPreset != 1 || m.Document.WidthMM != 40 || m.Document.HeightMM != 12 {
		t.Fatalf("loaded design = index %d %.1fx%.1f, want index 1 40x12", m.DesignPreset, m.Document.WidthMM, m.Document.HeightMM)
	}
}

func TestHelpAndMenuRenderAsModalViews(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Ready = true
	m.Width = minTerminalWidth
	m.Height = minTerminalHeight
	m.reflow()
	m.HelpOpen = true
	help := m.View()
	if !strings.Contains(help, "Show focus hints") {
		t.Fatal("help modal content missing")
	}
	m.HelpOpen = false
	m.MenuOpen = true
	menu := m.View()
	if !strings.Contains(menu, "Auto Insert on Text/QR Creation") || !strings.Contains(menu, "Label roll") || !strings.Contains(menu, "Saved preset") {
		t.Fatal("menu modal content missing")
	}
}

func TestSSavesDesignPromptInsteadOfSwitchingPrinter(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	if !m.handleCommandKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}) {
		t.Fatal("s key was not handled")
	}
	if m.Prompt.Mode != PromptSaveDesign {
		t.Fatalf("prompt mode = %q, want save design", m.Prompt.Mode)
	}
}

func TestSmallTerminalWarning(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Ready = true
	m.Width = minTerminalWidth - 1
	m.Height = minTerminalHeight
	view := m.View()
	if !strings.Contains(view, "Terminal size too small") || !strings.Contains(view, "Width = 140") {
		t.Fatal("small terminal warning missing required text")
	}
}

func TestFontPickerClosesOnOutsideClick(t *testing.T) {
	m := NewModel(50, 30, "rect", "", PrintConfig{})
	m.Canvas = newCanvas(80, 24, m.Document.WidthMM, m.Document.HeightMM)
	m.FontPickerOpen = true
	m.FontPickerSearch = true
	m.FontPickerQuery = "jet"

	m.handleMousePressAt(leftClick(0, 0), time.Now())
	if m.FontPickerOpen || m.FontPickerSearch || m.FontPickerQuery != "" {
		t.Fatalf("font picker state = open %t search %t query %q, want closed/cleared", m.FontPickerOpen, m.FontPickerSearch, m.FontPickerQuery)
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
