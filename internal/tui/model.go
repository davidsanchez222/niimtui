package tui

import (
	"context"
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/api"
	"niimtui/internal/label"
	"niimtui/internal/render"
)

type PrintService interface {
	PrintImage(ctx context.Context, selector string, rendered render.Result, copies int) api.PrintResponse
}

type PrintConfig struct {
	Service PrintService
	Printer string
	Model   string
	Copies  int
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

type Model struct {
	Width  int
	Height int

	Document label.Document
	Canvas   Canvas

	SelectedID string
	Drag       DragState
	NextID     int
	FontPath   string
	Print      PrintConfig

	EditingText bool
	TextBuffer  string
	StatusBase  string

	Status string
	Ready  bool
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
		18,
	)
	sample.Text.FontPath = fontPath
	clampElementToDocument(&sample, doc)
	_ = doc.AddElement(sample)
	status := "Click to select. Drag to move. Drag handles to resize."

	return Model{
		Document:   doc,
		SelectedID: sample.ID,
		NextID:     2,
		FontPath:   fontPath,
		Print:      printConfig,
		StatusBase: status,
		Status:     status,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
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
