package tui

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

type tuiTab uint8

const (
	tabDesigner tuiTab = iota
	tabGallery
)

type galleryItem struct {
	Name     string
	Document label.Document
	Bindings []config.DesignBinding
	Saved    bool
	Stock    string
}

type galleryRenderedMsg struct {
	Seq int
	PNG []byte
	Err error
}

func (m Model) galleryItems() []galleryItem {
	items := make([]galleryItem, 0, len(m.DesignPresets)+len(m.AllPresets)*3)
	stocks := m.AllPresets
	if len(stocks) == 0 {
		stocks = []config.LabelPreset{{Name: "custom", WidthMM: m.Document.WidthMM, HeightMM: m.Document.HeightMM, Shape: m.Document.Shape}}
	}
	for _, stock := range stocks {
		for _, kind := range []string{"QR", "QR + title", "Title + details"} {
			if kind == "QR + title" && math.Min(stock.WidthMM, stock.HeightMM) < 24 {
				continue
			}
			items = append(items, galleryItem{Name: stock.Name + " / " + kind, Document: starterDocument(stock, kind), Stock: stock.Name})
		}
	}
	for _, saved := range m.DesignPresets {
		items = append(items, galleryItem{Name: saved.Name, Document: saved.Document, Bindings: saved.Bindings, Saved: true})
	}
	return items
}

func starterDocument(stock config.LabelPreset, kind string) label.Document {
	doc := label.NewDocument(stock.WidthMM, stock.HeightMM)
	doc.Shape = stock.Shape
	w, h := stock.WidthMM, stock.HeightMM
	margin := math.Max(1, math.Min(w, h)*0.07)
	textSize := math.Max(10, math.Min(18, h*0.55))
	switch kind {
	case "QR":
		size := math.Min(w, h) * 0.64
		doc.Elements = append(doc.Elements, label.NewQRElement("qr-1", "https://example.org", (w-size)/2, (h-size)/2, size))
	case "QR + title":
		size := math.Min(w*0.7, h*0.57)
		doc.Elements = append(doc.Elements, label.NewQRElement("qr-1", "https://example.org", (w-size)/2, margin, size))
		doc.Elements = append(doc.Elements, label.NewTextElement("text-2", "Item title", margin, h*0.69, w-2*margin, h*0.22, textSize))
	default:
		doc.Elements = append(doc.Elements,
			label.NewTextElement("text-1", "Item title", margin, h*0.17, w-2*margin, h*0.3, textSize),
			label.NewTextElement("text-2", "Details", margin, h*0.55, w-2*margin, h*0.24, math.Max(9, textSize*0.8)),
		)
	}
	return doc
}

func (m Model) currentGalleryItem() (galleryItem, bool) {
	items := m.galleryItems()
	if m.GalleryIndex < 0 || m.GalleryIndex >= len(items) {
		return galleryItem{}, false
	}
	return items[m.GalleryIndex], true
}

func (m *Model) switchTab(tab tuiTab) tea.Cmd {
	if m.Tab == tab {
		return nil
	}
	m.Tab = tab
	m.SidebarFocused = false
	m.setStatus("Label gallery: arrows browse, Enter opens design, c shows saved print command.")
	if tab == tabGallery {
		cmd := m.requestGalleryPreview()
		if m.Preview.Protocol == LivePreviewKitty {
			return tea.Batch(clearTerminalLivePreviewCmd(m.canvasPanelWidth()), cmd)
		}
		return cmd
	}
	m.setStatus("Label designer focused.")
	if m.Preview.Protocol == LivePreviewKitty && len(m.Preview.PNG) > 0 {
		m.Preview.RedrawSeq++
		return livePreviewRedrawCmd(m.Preview.RedrawSeq)
	}
	if m.Preview.Protocol == LivePreviewKitty {
		return clearTerminalLivePreviewCmd(m.canvasPanelWidth())
	}
	return nil
}

func (m *Model) handleGalleryKey(key tea.KeyMsg) tea.Cmd {
	items := m.galleryItems()
	switch key.String() {
	case "up", "k", "down", "j":
		if len(items) == 0 {
			return nil
		}
		step := 1
		if key.String() == "up" || key.String() == "k" {
			step = -1
		}
		m.GalleryIndex = (m.GalleryIndex + step + len(items)) % len(items)
		cmd := m.requestGalleryPreview()
		if m.Preview.Protocol == LivePreviewKitty {
			return tea.Batch(clearTerminalLivePreviewCmd(m.canvasPanelWidth()), cmd)
		}
		return cmd
	case "enter":
		return m.openGalleryItem()
	case "c":
		m.showGalleryCommand()
	}
	return nil
}

func (m *Model) requestGalleryPreview() tea.Cmd {
	item, ok := m.currentGalleryItem()
	if !ok {
		m.GalleryPNG = nil
		return nil
	}
	m.GallerySeq++
	m.GalleryPNG = nil
	m.GalleryErr = ""
	seq := m.GallerySeq
	printConfig := m.Print
	compatible := m.galleryCompatible(item)
	return func() tea.Msg {
		result, err := render.RenderDocument(item.Document)
		if err == nil && compatible {
			result, err = preparePrintPreviewResult(result, printConfig)
		}
		if err != nil {
			return galleryRenderedMsg{Seq: seq, Err: err}
		}
		return galleryRenderedMsg{Seq: seq, PNG: result.PreviewPNG}
	}
}

func (m Model) galleryCompatible(item galleryItem) bool {
	for _, stock := range m.Presets {
		if config.MatchesStock(item.Document, stock) {
			return true
		}
	}
	return m.Print.Model == ""
}

func (m *Model) onGalleryRendered(msg galleryRenderedMsg) tea.Cmd {
	if msg.Seq != m.GallerySeq {
		return nil
	}
	if msg.Err != nil {
		m.GalleryErr = msg.Err.Error()
		return nil
	}
	m.GalleryPNG = msg.PNG
	if m.Tab == tabGallery && m.Preview.Protocol == LivePreviewKitty && !m.MenuOpen && !m.HelpOpen && !m.confirmPromptOpen() {
		return galleryTerminalPreviewCmd(*m)
	}
	return nil
}

func (m *Model) openGalleryItem() tea.Cmd {
	item, ok := m.currentGalleryItem()
	if !ok {
		return nil
	}
	if m.unsavedTemplate || !reflect.DeepEqual(m.Document, m.cleanDocument) || !reflect.DeepEqual(m.Bindings, m.cleanBindings) {
		m.Prompt = PromptState{Mode: PromptOpenGallery, Value: item.Name}
		m.refreshPromptStatus()
		return nil
	}
	return m.loadGalleryItem(item)
}

func (m *Model) loadGalleryItem(item galleryItem) tea.Cmd {
	if item.Saved {
		for i, saved := range m.DesignPresets {
			if saved.Name == item.Name {
				m.loadDesignPreset(i)
				m.cleanDocument = cloneDocument(m.Document)
				m.cleanBindings = append([]config.DesignBinding(nil), m.Bindings...)
				m.unsavedTemplate = false
				break
			}
		}
	} else {
		m.Document = cloneDocument(item.Document)
		m.Bindings = nil
		m.DesignPreset = -1
		m.Preset = activePresetIndex(m.Presets, "", m.Document)
		m.NextID = nextIDForDocument(m.Document)
		m.SelectedID = ""
		m.Drag = DragState{}
		m.reflow()
		m.commitHistory("open starter label")
		m.unsavedTemplate = true
		m.setStatus("Opened %q in designer. Save it to use from CLI.", item.Name)
	}
	cmd := m.switchTab(tabDesigner)
	if item.Saved {
		m.setStatus("Loaded saved design %q in designer.", item.Name)
	} else {
		m.setStatus("Opened %q in designer. Save it to use from CLI.", item.Name)
	}
	return cmd
}

func galleryTerminalPreviewCmd(m Model) tea.Cmd {
	if len(m.GalleryPNG) == 0 || m.Tab != tabGallery || m.MenuOpen || m.HelpOpen || m.confirmPromptOpen() {
		return clearTerminalLivePreviewCmd(m.canvasPanelWidth())
	}
	png := append([]byte(nil), m.GalleryPNG...)
	cols, rows := m.galleryPreviewCellSize(layoutPropertiesWidth)
	return func() tea.Msg {
		_, _ = writeTerminalLivePreview(os.Stdout, m.Preview.Protocol, png, m.canvasPanelWidth(), cols, rows)
		return nil
	}
}

func (m Model) galleryPreviewCellSize(width int) (int, int) {
	item, ok := m.currentGalleryItem()
	if !ok {
		return m.livePreviewPanelCellSize(width)
	}
	return previewPanelCellSize(width, item.Document.WidthMM, item.Document.HeightMM)
}

func galleryListLines(m Model, width int) []string {
	items := m.galleryItems()
	lines := []string{propertyTitleStyle.Render("Label gallery"), mutedStyle.Render("↑/↓ browse · enter edit"), ""}
	if len(items) == 0 {
		return append(lines, mutedStyle.Render("No designs available"))
	}
	start, end := sidebarWindow(len(items), m.GalleryIndex, max(1, m.canvasPanelHeight()-7))
	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		if i == m.GalleryIndex {
			prefix = "> "
		}
		mark := " "
		if !m.galleryCompatible(item) {
			mark = "!"
		}
		lines = append(lines, truncateText(prefix+mark+" "+item.Name, width))
	}
	lines = append(lines, "", mutedStyle.Render("! = no matching installed roll"), helpItem("c", "saved print command"))
	return padLines(lines, width)
}

func (m Model) galleryPreviewLines(width int) []string {
	item, ok := m.currentGalleryItem()
	if !ok {
		return padLines([]string{"No label selected"}, width)
	}
	source := "Starter design"
	if item.Saved {
		source = "Saved design"
	}
	lines := []string{propertyTitleStyle.Render("Preview"), ""}
	if m.Preview.Protocol == LivePreviewKitty && len(m.GalleryPNG) > 0 {
		_, rows := m.galleryPreviewCellSize(width)
		for range rows {
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, mutedStyle.Render("Canvas preview in center"))
	}
	lines = append(lines, "", propertyItem("Name", truncateText(item.Name, sidebarValueWidth("Name", width))), propertyItem("Type", source), propertyItem("Size", fmt.Sprintf("%.0f × %.0f mm", item.Document.WidthMM, item.Document.HeightMM)), propertyItem("Shape", item.Document.Shape))
	if !m.galleryCompatible(item) {
		lines = append(lines, mutedStyle.Render("No matching installed roll"))
	}
	if m.GalleryErr != "" {
		lines = append(lines, mutedStyle.Render("Preview: "+m.GalleryErr))
	}
	return padLines(lines, width)
}

func galleryCanvasLines(m Model, width, height int) []string {
	item, ok := m.currentGalleryItem()
	if !ok {
		return fitPanelLines([]string{"No labels available"}, height, width)
	}
	preview := m
	preview.Document = item.Document
	preview.SelectedID = ""
	preview.Grid = false
	preview.FocusPickerOpen = false
	preview.reflow()
	return fitPanelLines(centerCanvasLines(strings.Split(renderCanvas(preview), "\n"), width, height), height, width)
}
