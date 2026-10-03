package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/label"
	"niimtui/internal/printtrace"
	"niimtui/internal/render"
	"niimtui/internal/transport"
)

type PrinterSession interface {
	Connect(ctx context.Context) (map[string]any, error)
	PrintImage(ctx context.Context, rendered render.Result, copies int) api.PrintResponse
	Metadata() map[string]any
	Close() error
}

type PrinterSessionFactory func(selector string) (PrinterSession, error)
type PrinterDiscovery func(ctx context.Context) ([]transport.ScanResult, error)

type PrintConfig struct {
	Session          PrinterSession
	NewSession       PrinterSessionFactory
	Discover         PrinterDiscovery
	DiscoveryContext context.Context
	PreferredPrinter string
	ExplicitPrinter  bool
	ConfigPath       string
	Printers         []config.PrinterProfile
	DesignPresets    []config.DesignPreset
	Printer          string
	Model            string
	DeviceName       string
	Identifier       string
	OffsetXMM        float64
	OffsetYMM        float64
	Copies           int
}

type ConnectionInfo struct {
	Meta map[string]any
}

type ConnectionStatus string

const (
	ConnectionUnavailable  ConnectionStatus = "unavailable"
	ConnectionConnecting   ConnectionStatus = "connecting"
	ConnectionConnected    ConnectionStatus = "connected"
	ConnectionDisconnected ConnectionStatus = "disconnected"
)

type printerConnectedMsg struct {
	Info ConnectionInfo
	Seq  int
}

type printerConnectionFailedMsg struct {
	Err error
	Seq int
}

type DragMode int

const (
	DragNone DragMode = iota
	DragMove
	DragResize
)

type ResizeHandle int

const (
	HandleNone ResizeHandle = iota
	HandleTopLeft
	HandleTop
	HandleTopRight
	HandleLeft
	HandleRight
	HandleBottomLeft
	HandleBottom
	HandleBottomRight
)

type DragState struct {
	Mode   DragMode
	Handle ResizeHandle

	StartMouseX int
	StartMouseY int

	OriginalElement label.Element
}

type ClickState struct {
	ElementID string
	X         int
	Y         int
	At        time.Time
}

type HistorySnapshot struct {
	Document   label.Document
	Bindings   []config.DesignBinding
	SelectedID string
	NextID     int
	Preset     int
}

type HistoryNode struct {
	Parent   *HistoryNode
	Children []*HistoryNode
	Label    string
	Snapshot HistorySnapshot
}

type PromptMode string

const (
	PromptNone            PromptMode = ""
	PromptExportPNG       PromptMode = "export-png"
	PromptSaveDesign      PromptMode = "save-design"
	PromptBinding         PromptMode = "binding"
	PromptOverwriteDesign PromptMode = "overwrite-design"
	PromptDeleteDesign    PromptMode = "delete-design"
	PromptOpenGallery     PromptMode = "open-gallery"
	PromptCommand         PromptMode = "print-command"
)

type PromptState struct {
	Mode   PromptMode
	Value  string
	Choice int
}

type Model struct {
	Width            int
	Height           int
	Tab              tuiTab
	GalleryIndex     int
	GalleryCollapsed [3]bool
	galleryStarters  []galleryItem
	GallerySeq       int
	GalleryPNG       []byte
	GalleryErr       string
	CommandScroll    int

	Document      label.Document
	Canvas        Canvas
	AllPresets    []config.LabelPreset
	Presets       []config.LabelPreset
	Preset        int
	DesignPresets []config.DesignPreset
	DesignPreset  int
	Bindings      []config.DesignBinding
	Grid          bool
	// Count is a pending vim-style move count typed before h/j/k/l or an arrow.
	Count int

	SelectedID string
	Drag       DragState
	LastClick  ClickState
	NextID     int
	Clipboard  []label.Element
	History    *HistoryNode
	RedoBranch int
	FontPath   string
	Fonts      []FontOption

	FontPickerOpen   bool
	FontPickerSearch bool
	FontPickerIndex  int
	FontPickerQuery  string
	FontPreviewDoc   *label.Document
	FontPreviewIndex int
	FontPreviewID    string

	FocusPickerOpen  bool
	HelpOpen         bool
	MenuOpen         bool
	MenuIndex        int
	AutoInsert       bool
	SidebarFocused   bool
	SidebarIndex     int
	SidebarCollapsed map[string]bool
	TopBarHover      topBarTarget

	Print           PrintConfig
	Connection      ConnectionStatus
	ConnectSeq      int
	ConnectErr      string
	ConnectMeta     map[string]any
	Discovering     bool
	DetectedNames   map[string]bool
	PendingConnect  bool
	ScanSeq         int
	scanCtx         context.Context
	scanCancel      context.CancelFunc
	startupDocument label.Document
	cleanDocument   label.Document
	cleanBindings   []config.DesignBinding
	unsavedTemplate bool

	EditingText bool
	TextBuffer  string
	Prompt      PromptState
	StatusBase  string

	Status string
	Ready  bool

	Preview   LivePreviewState
	Selection textSelection
}

type LivePreviewProtocol string

const (
	LivePreviewDisabled LivePreviewProtocol = ""
	LivePreviewKitty    LivePreviewProtocol = "kitty"
	// LivePreviewOpen refreshes a native Preview window for terminals without inline images.
	LivePreviewOpen LivePreviewProtocol = "open"
)

type LivePreviewState struct {
	Protocol     LivePreviewProtocol
	RequestedSeq int
	RenderedSeq  int
	PNG          []byte
	PNGHash      string
	LastOpenHash string
	LastKey      string
	RedrawSeq    int
	// RedrawPending throttles stale-image redraws while the document changes faster than it renders.
	RedrawPending bool
	Err           string
}

func NewModel(widthMM, heightMM float64, shape, fontPath string, printConfig PrintConfig) Model {
	return NewModelWithPresets(widthMM, heightMM, shape, fontPath, printConfig, nil, "")
}

func NewModelWithPresets(widthMM, heightMM float64, shape, fontPath string, printConfig PrintConfig, presets []config.LabelPreset, presetName string) Model {
	doc := label.NewDocument(widthMM, heightMM)
	if strings.TrimSpace(shape) != "" {
		doc.Shape = strings.ToLower(strings.TrimSpace(shape))
	}
	status := "Click to select. Canvas drag edits; drag panel text to highlight and copy."
	if printConfig.Discover != nil {
		status = "Scanning for configured printers. You can start designing now."
	}
	preview := LivePreviewState{Protocol: detectLivePreviewProtocol()}
	if preview.Protocol != LivePreviewDisabled {
		preview.RequestedSeq = 1
		preview.LastKey = documentPreviewKey(doc, printConfig)
		status = fmt.Sprintf("Realtime preview: %s. %s", livePreviewProtocolLabel(preview.Protocol), status)
	}
	connection := ConnectionUnavailable
	if printConfig.Session != nil {
		connection = ConnectionConnecting
	} else if printConfig.NewSession != nil {
		connection = ConnectionDisconnected
	}
	var scanCtx context.Context
	var scanCancel context.CancelFunc
	if printConfig.Discover != nil {
		parent := printConfig.DiscoveryContext
		if parent == nil {
			parent = context.Background()
		}
		scanCtx, scanCancel = context.WithCancel(parent)
	}

	fonts := discoverFonts(fontPath)
	presets = validTUIPresets(presets)
	visiblePresets := presetsForPrinter(presets, printConfig.Model)
	m := Model{
		Document:        doc,
		Tab:             tabDesigner,
		cleanDocument:   cloneDocument(doc),
		AllPresets:      presets,
		Presets:         visiblePresets,
		Preset:          activePresetIndex(visiblePresets, presetName, doc),
		DesignPresets:   cloneDesignPresets(printConfig.DesignPresets),
		DesignPreset:    -1,
		NextID:          1,
		FontPath:        fontPath,
		Fonts:           fonts,
		FontPickerIndex: fontOptionIndex(fonts, fontPath),
		Print:           printConfig,
		Connection:      connection,
		Discovering:     printConfig.Discover != nil,
		ScanSeq:         1,
		scanCtx:         scanCtx,
		scanCancel:      scanCancel,
		startupDocument: cloneDocument(doc),
		StatusBase:      status,
		Status:          status,
		Preview:         preview,
	}
	m.galleryStarters = starterGalleryItems(m.AllPresets)
	for i, row := range m.galleryRows() {
		if !row.Header && !row.Empty {
			m.GalleryIndex = i
			break
		}
	}
	m.initHistory()
	return m
}

func presetsForPrinter(presets []config.LabelPreset, model string) []config.LabelPreset {
	model = strings.TrimSpace(model)
	if model == "" {
		return presets
	}
	filtered := make([]config.LabelPreset, 0, len(presets))
	prefix := printerPresetPrefix(model)
	for _, preset := range presets {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(preset.Name)), prefix) {
			filtered = append(filtered, preset)
		}
	}
	return filtered
}

func printerPresetPrefix(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.ReplaceAll(model, " ", "-")
	return model + "-"
}

func validTUIPresets(presets []config.LabelPreset) []config.LabelPreset {
	valid := make([]config.LabelPreset, 0, len(presets))
	for _, preset := range presets {
		if preset.WidthMM <= 0 || preset.HeightMM <= 0 {
			continue
		}
		valid = append(valid, preset)
	}
	return valid
}

func activePresetIndex(presets []config.LabelPreset, name string, doc label.Document) int {
	name = strings.TrimSpace(name)
	if name != "" {
		for i, preset := range presets {
			if preset.Name == name {
				return i
			}
		}
	}
	for i, preset := range presets {
		if preset.WidthMM == doc.WidthMM && preset.HeightMM == doc.HeightMM && strings.EqualFold(preset.Shape, doc.Shape) {
			return i
		}
	}
	return -1
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.Print.Discover != nil {
		cmds = append(cmds, printerScanCmd(m.Print.Discover, m.scanCtx, m.ScanSeq))
	}
	if m.Print.Session == nil {
		if m.Preview.Protocol != LivePreviewDisabled && m.Preview.RequestedSeq > 0 {
			cmds = append(cmds, livePreviewDebounceCmd(m.Preview.RequestedSeq))
		}
		return tea.Batch(cmds...)
	}
	cmds = append(cmds, connectPrinterCmd(m.Print.Session, m.ConnectSeq))
	if m.Preview.Protocol != LivePreviewDisabled && m.Preview.RequestedSeq > 0 {
		cmds = append(cmds, livePreviewDebounceCmd(m.Preview.RequestedSeq))
	}
	return tea.Batch(cmds...)
}

func (m *Model) setStatus(format string, args ...any) {
	if len(args) == 0 {
		m.StatusBase = format
	} else {
		m.StatusBase = fmt.Sprintf(format, args...)
	}
	m.refreshStatus()
}

func (m *Model) refreshStatus() {
	if m.EditingText {
		m.Status = fmt.Sprintf("Editing %s.", m.editingTargetLabel())
		return
	}
	m.Status = m.StatusBase
}

func (m Model) editingTargetLabel() string {
	element, ok := m.selectedElement()
	if ok && element.QR != nil {
		return "QR"
	}
	return "text"
}

func connectPrinterCmd(session PrinterSession, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx := printtrace.Start(context.Background())
		printtrace.Mark(ctx, "TUI connect requested")
		meta, err := session.Connect(ctx)
		printtrace.Mark(ctx, "TUI connect completed")
		if err != nil {
			return printerConnectionFailedMsg{Err: err, Seq: seq}
		}
		return printerConnectedMsg{Info: ConnectionInfo{Meta: meta}, Seq: seq}
	}
}

func detectLivePreviewProtocol() LivePreviewProtocol {
	termProgram := strings.ToLower(strings.TrimSpace(envValue("TERM_PROGRAM")))
	term := strings.ToLower(strings.TrimSpace(envValue("TERM")))
	switch strings.ToLower(envValue("NIIMTUI_GRAPHICS")) {
	case "kitty":
		return LivePreviewKitty
	case "open":
		if openPreviewAvailable() {
			return LivePreviewOpen
		}
		return LivePreviewDisabled
	case "off", "none", "disabled":
		return LivePreviewDisabled
	}
	if envValue("KITTY_WINDOW_ID") != "" || strings.Contains(term, "xterm-kitty") || termProgram == "ghostty" {
		return LivePreviewKitty
	}
	// WezTerm implements the kitty graphics protocol (enable_kitty_graphics) but identifies as xterm-256color.
	// The WEZTERM_* variables survive tmux, which rewrites TERM_PROGRAM.
	if termProgram == "wezterm" || envValue("WEZTERM_PANE") != "" || envValue("WEZTERM_EXECUTABLE") != "" {
		return LivePreviewKitty
	}
	if openPreviewAvailable() {
		return LivePreviewOpen
	}
	return LivePreviewDisabled
}

var envValue = func(key string) string {
	return strings.TrimSpace(getenv(key))
}

var getenv = func(key string) string {
	return os.Getenv(key)
}

func livePreviewProtocolLabel(protocol LivePreviewProtocol) string {
	switch protocol {
	case LivePreviewKitty:
		return "terminal image"
	case LivePreviewOpen:
		return "Preview window"
	default:
		return "disabled"
	}
}
