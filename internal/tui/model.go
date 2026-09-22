package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/api"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

type PrinterSession interface {
	Connect(ctx context.Context) (map[string]any, error)
	PrintImage(ctx context.Context, rendered render.Result, copies int) api.PrintResponse
	Close() error
}

type PrintConfig struct {
	Session    PrinterSession
	Printer    string
	Model      string
	DeviceName string
	Identifier string
	OffsetXMM  float64
	OffsetYMM  float64
	Copies     int
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
}

type printerConnectionFailedMsg struct {
	Err error
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

type Model struct {
	Width  int
	Height int

	Document label.Document
	Canvas   Canvas

	SelectedID string
	Drag       DragState
	LastClick  ClickState
	NextID     int
	FontPath   string
	Fonts      []FontOption

	FontPickerOpen   bool
	FontPickerSearch bool
	FontPickerIndex  int
	FontPickerQuery  string

	FocusPickerOpen bool
	HelpOpen        bool
	MenuOpen        bool
	MenuIndex       int
	AutoInsert      bool

	Print       PrintConfig
	Connection  ConnectionStatus
	ConnectErr  string
	ConnectMeta map[string]any

	PrinterArtVariant int

	EditingText bool
	TextBuffer  string
	StatusBase  string

	Status string
	Ready  bool

	Preview LivePreviewState
}

type LivePreviewProtocol string

const (
	LivePreviewDisabled LivePreviewProtocol = ""
	LivePreviewKitty    LivePreviewProtocol = "kitty"
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
	Err          string
}

func NewModel(widthMM, heightMM float64, shape, fontPath string, printConfig PrintConfig) Model {
	doc := label.NewDocument(widthMM, heightMM)
	if strings.TrimSpace(shape) != "" {
		doc.Shape = strings.ToLower(strings.TrimSpace(shape))
	}
	sample := label.NewTextElement(
		"text-1",
		"Storage Box 12",
		math.Max(widthMM*0.15, 2),
		math.Max(heightMM*0.20, 2),
		math.Max(math.Min(widthMM*0.45, widthMM-4), 10),
		math.Max(math.Min(heightMM*0.22, heightMM-4), 6),
		40,
	)
	sample.Text.FontPath = fontPath
	clampElementToDocument(&sample, doc)
	_ = doc.AddElement(sample)
	status := "Click to select. Drag to move. Drag handles to resize."
	preview := LivePreviewState{Protocol: detectLivePreviewProtocol()}
	if preview.Protocol != LivePreviewDisabled {
		preview.RequestedSeq = 1
		preview.LastKey = documentPreviewKey(doc, printConfig)
		status = fmt.Sprintf("Realtime preview: %s. %s", livePreviewProtocolLabel(preview.Protocol), status)
	}
	connection := ConnectionUnavailable
	if printConfig.Session != nil {
		connection = ConnectionConnecting
	}

	fonts := discoverFonts(fontPath)
	return Model{
		Document:        doc,
		SelectedID:      sample.ID,
		NextID:          2,
		FontPath:        fontPath,
		Fonts:           fonts,
		FontPickerIndex: fontOptionIndex(fonts, fontPath),
		Print:           printConfig,
		Connection:      connection,
		StatusBase:      status,
		Status:          status,
		Preview:         preview,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.Print.Session == nil {
		if m.Preview.Protocol != LivePreviewDisabled && m.Preview.RequestedSeq > 0 {
			return livePreviewDebounceCmd(m.Preview.RequestedSeq)
		}
		return nil
	}
	cmds = append(cmds, connectPrinterCmd(m.Print.Session))
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
		m.Status = fmt.Sprintf("Editing: %s", m.TextBuffer)
		return
	}
	m.Status = m.StatusBase
}

func connectPrinterCmd(session PrinterSession) tea.Cmd {
	return func() tea.Msg {
		meta, err := session.Connect(context.Background())
		if err != nil {
			return printerConnectionFailedMsg{Err: err}
		}
		return printerConnectedMsg{Info: ConnectionInfo{Meta: meta}}
	}
}

func detectLivePreviewProtocol() LivePreviewProtocol {
	termProgram := strings.ToLower(strings.TrimSpace(envValue("TERM_PROGRAM")))
	term := strings.ToLower(strings.TrimSpace(envValue("TERM")))
	if envValue("KITTY_WINDOW_ID") != "" || strings.Contains(term, "xterm-kitty") || termProgram == "ghostty" {
		return LivePreviewKitty
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
	default:
		return "disabled"
	}
}
