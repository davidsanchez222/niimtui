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
	Group    galleryGroup
}

type galleryGroup uint8

const (
	galleryB1 galleryGroup = iota
	galleryD110
	galleryCustom
)

func (group galleryGroup) title() string {
	switch group {
	case galleryB1:
		return "B1"
	case galleryD110:
		return "D110"
	default:
		return "Custom"
	}
}

type galleryRow struct {
	Group  galleryGroup
	Header bool
	Empty  bool
	Item   galleryItem
}

type galleryRenderedMsg struct {
	Seq int
	PNG []byte
	Err error
}

func (m Model) galleryItems() []galleryItem {
	items := make([]galleryItem, 0, len(m.DesignPresets)+len(m.galleryStarters))
	items = append(items, m.galleryStarters...)
	for _, saved := range m.DesignPresets {
		items = append(items, galleryItem{Name: saved.Name, Document: saved.Document, Bindings: saved.Bindings, Saved: true, Group: galleryCustom})
	}
	return items
}

func starterGalleryItems(stocks []config.LabelPreset) []galleryItem {
	items := make([]galleryItem, 0, len(stocks)*3)
	for _, stock := range stocks {
		var group galleryGroup
		switch {
		case strings.HasPrefix(strings.ToLower(stock.Name), "b1-"):
			group = galleryB1
		case strings.HasPrefix(strings.ToLower(stock.Name), "d110-"):
			group = galleryD110
		default:
			continue
		}
		for _, kind := range []string{"QR", "QR + title", "Title + details"} {
			doc := starterDocument(stock, kind)
			if !starterTextFits(doc) {
				continue
			}
			items = append(items, galleryItem{Name: stock.Name + " / " + kind, Document: doc, Stock: stock.Name, Group: group})
		}
	}
	return items
}

func starterTextFits(doc label.Document) bool {
	for _, element := range doc.Elements {
		if element.Text == nil {
			continue
		}
		height, err := render.RequiredTextHeightMM(element, element.WidthMM)
		if err != nil || height > element.HeightMM {
			return false
		}
	}
	return true
}

func (m Model) galleryRows() []galleryRow {
	items := m.galleryItems()
	rows := make([]galleryRow, 0, len(items)+3)
	for _, group := range []galleryGroup{galleryB1, galleryD110, galleryCustom} {
		rows = append(rows, galleryRow{Group: group, Header: true})
		if m.GalleryCollapsed[group] {
			continue
		}
		count := 0
		for _, item := range items {
			if item.Group == group {
				rows = append(rows, galleryRow{Group: group, Item: item})
				count++
			}
		}
		if count == 0 {
			rows = append(rows, galleryRow{Group: group, Empty: true})
		}
	}
	return rows
}

func (m Model) currentGalleryRow() (galleryRow, bool) {
	rows := m.galleryRows()
	if m.GalleryIndex < 0 || m.GalleryIndex >= len(rows) {
		return galleryRow{}, false
	}
	return rows[m.GalleryIndex], true
}

func starterDocument(stock config.LabelPreset, kind string) label.Document {
	doc := label.NewDocument(stock.WidthMM, stock.HeightMM)
	doc.Shape = stock.Shape
	w, h := stock.WidthMM, stock.HeightMM
	margin := math.Max(1, math.Min(w, h)*0.07)
	textSize := 40.0
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
			label.NewTextElement("text-2", "Details", margin, h*0.55, w-2*margin, h*0.24, textSize),
		)
	}
	return doc
}

func (m Model) currentGalleryItem() (galleryItem, bool) {
	row, ok := m.currentGalleryRow()
	if !ok || row.Header || row.Empty {
		return galleryItem{}, false
	}
	return row.Item, true
}

func (m *Model) toggleGalleryGroup(group galleryGroup, collapsed bool) {
	if m.GalleryCollapsed[group] == collapsed {
		return
	}
	selected, _ := m.currentGalleryRow()
	m.GalleryCollapsed[group] = collapsed
	rows := m.galleryRows()
	for i, row := range rows {
		if (selected.Group == group && collapsed && row.Header && row.Group == group) ||
			(row.Header == selected.Header && row.Empty == selected.Empty && row.Group == selected.Group && (row.Header || row.Item.Name == selected.Item.Name)) {
			m.GalleryIndex = i
			break
		}
	}
}

func (m *Model) reconcileGalleryAfterDelete(_ string) {
	m.GalleryPNG = nil
	m.GallerySeq++
	for i, row := range m.galleryRows() {
		if row.Group == galleryCustom && !row.Header && !row.Empty {
			m.GalleryIndex = i
			return
		}
		if row.Group == galleryCustom && row.Header {
			m.GalleryIndex = i
		}
	}
}

func (m *Model) switchTab(tab tuiTab) tea.Cmd {
	if m.Tab == tab {
		return nil
	}
	m.Tab = tab
	m.SidebarFocused = false
	m.setStatus("Label gallery: ↑/↓ or k/j browse, ←/→ or h/l fold, Enter opens, c shows saved print command.")
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
	rows := m.galleryRows()
	switch key.String() {
	case "up", "k", "down", "j":
		if len(rows) == 0 {
			return nil
		}
		step := 1
		if key.String() == "up" || key.String() == "k" {
			step = -1
		}
		for i := 0; i < len(rows); i++ {
			m.GalleryIndex = (m.GalleryIndex + step + len(rows)) % len(rows)
			if !rows[m.GalleryIndex].Empty {
				break
			}
		}
		cmd := m.requestGalleryPreview()
		if m.Preview.Protocol == LivePreviewKitty {
			return tea.Batch(clearTerminalLivePreviewCmd(m.canvasPanelWidth()), cmd)
		}
		return cmd
	case "left", "h", "right", "l", " ":
		row, ok := m.currentGalleryRow()
		if !ok {
			return nil
		}
		collapsed := key.String() == "left" || key.String() == "h"
		if key.String() == " " {
			collapsed = !m.GalleryCollapsed[row.Group]
		}
		m.toggleGalleryGroup(row.Group, collapsed)
		return m.requestGalleryPreview()
	case "enter":
		if row, ok := m.currentGalleryRow(); ok && row.Header {
			m.toggleGalleryGroup(row.Group, !m.GalleryCollapsed[row.Group])
			return m.requestGalleryPreview()
		}
		return m.openGalleryItem()
	case "d":
		if item, ok := m.currentGalleryItem(); ok && item.Saved {
			m.Prompt = PromptState{Mode: PromptDeleteDesign, Value: item.Name}
			m.refreshPromptStatus()
		}
	case "c":
		m.showGalleryCommand()
	}
	return nil
}

func (m *Model) requestGalleryPreview() tea.Cmd {
	item, ok := m.currentGalleryItem()
	if !ok {
		m.GallerySeq++
		m.GalleryPNG = nil
		m.GalleryErr = ""
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
	items := m.galleryRows()
	lines := []string{propertyTitleStyle.Render("Label gallery"), mutedStyle.Render("↑/↓ or k/j browse"), ""}
	start, end := sidebarWindow(len(items), m.GalleryIndex, max(1, m.canvasPanelHeight()-7))
	for i := start; i < end; i++ {
		row := items[i]
		prefix := "  "
		if i == m.GalleryIndex {
			prefix = "> "
		}
		if row.Header {
			arrow := "▾ "
			if m.GalleryCollapsed[row.Group] {
				arrow = "▸ "
			}
			lines = append(lines, propertyTitleStyle.Render(prefix+arrow+row.Group.title()))
			continue
		}
		if row.Empty {
			lines = append(lines, mutedStyle.Render("    No labels configured"))
			continue
		}
		item := row.Item
		mark := " "
		if !m.galleryCompatible(item) {
			mark = "!"
		}
		lines = append(lines, truncateText(prefix+mark+" "+strings.TrimPrefix(item.Name, item.Stock+" / "), width))
	}
	lines = append(lines, "", mutedStyle.Render("! = no matching roll"), helpItem("c", "command")+"  "+helpItem("d", "delete"))
	return padLines(lines, width)
}

func (m Model) galleryPreviewLines(width int) []string {
	item, ok := m.currentGalleryItem()
	if !ok {
		if row, selected := m.currentGalleryRow(); selected && row.Header {
			return padLines([]string{propertyTitleStyle.Render(row.Group.title()), "", mutedStyle.Render("↑/↓ or k/j browse"), mutedStyle.Render("←/→ or h/l fold")}, width)
		}
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
