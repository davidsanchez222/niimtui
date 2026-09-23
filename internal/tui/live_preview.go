package tui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/label"
	"niimtui/internal/render"
)

const livePreviewDebounce = 150 * time.Millisecond

const terminalLivePreviewImageID = 4242

type livePreviewTickMsg struct {
	Seq int
}

type livePreviewRenderedMsg struct {
	Seq      int
	PNG      []byte
	PNGHash  string
	WidthPx  int
	HeightPx int
}

type livePreviewFailedMsg struct {
	Seq int
	Err error
}

type livePreviewRedrawMsg struct {
	Seq int
}

func livePreviewDebounceCmd(seq int) tea.Cmd {
	return tea.Tick(livePreviewDebounce, func(time.Time) tea.Msg {
		return livePreviewTickMsg{Seq: seq}
	})
}

func livePreviewRedrawCmd(seq int) tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return livePreviewRedrawMsg{Seq: seq}
	})
}

func renderLivePreviewCmd(m Model, seq int) tea.Cmd {
	doc := m.Document
	printConfig := m.Print
	return func() tea.Msg {
		result, err := render.RenderDocument(doc)
		if err != nil {
			return livePreviewFailedMsg{Seq: seq, Err: fmt.Errorf("render label: %w", err)}
		}
		result, err = preparePrintPreviewResult(result, printConfig)
		if err != nil {
			return livePreviewFailedMsg{Seq: seq, Err: fmt.Errorf("fit preview: %w", err)}
		}
		return livePreviewRenderedMsg{
			Seq:      seq,
			PNG:      result.PreviewPNG,
			PNGHash:  pngHash(result.PreviewPNG),
			WidthPx:  result.WidthPx,
			HeightPx: result.HeightPx,
		}
	}
}

func (m Model) handleLivePreviewRendered(msg livePreviewRenderedMsg) (Model, tea.Cmd) {
	if msg.Seq != m.Preview.RequestedSeq || m.Preview.Protocol == LivePreviewDisabled {
		return m, nil
	}
	m.Preview.RenderedSeq = msg.Seq
	m.Preview.PNG = msg.PNG
	m.Preview.PNGHash = msg.PNGHash
	m.Preview.Err = ""
	if m.Preview.Protocol == LivePreviewKitty {
		return m, terminalLivePreviewCmd(m)
	}
	return m, nil
}

func preparePrintPreviewResult(result render.Result, printConfig PrintConfig) (render.Result, error) {
	if printConfig.Model == "" {
		return result, nil
	}
	fitted, err := render.FitToPrinterWidth(result, printConfig.Model)
	if err != nil {
		return render.Result{}, err
	}
	offsetX, offsetY := render.ModelPrintOffsetMM(printConfig.Model, printConfig.OffsetXMM, printConfig.OffsetYMM)
	return render.ApplyPrintOffset(fitted, offsetX, offsetY)
}

func openPreviewCmd(path string) tea.Cmd {
	return func() tea.Msg {
		_, _ = openPreviewFile(path)
		return nil
	}
}

func terminalLivePreviewCmd(m Model) tea.Cmd {
	if m.isTerminalTooSmall() {
		return clearTerminalLivePreviewCmd(m.Canvas.Width)
	}
	protocol := m.Preview.Protocol
	png := append([]byte(nil), m.Preview.PNG...)
	canvasWidth := m.Canvas.Width
	return func() tea.Msg {
		_, _ = writeTerminalLivePreview(os.Stdout, protocol, png, canvasWidth)
		return nil
	}
}

func clearTerminalLivePreviewCmd(canvasWidth int) tea.Cmd {
	return func() tea.Msg {
		_, _ = writeTerminalLivePreviewClear(os.Stdout, canvasWidth)
		return nil
	}
}

func writeTerminalLivePreview(w io.Writer, protocol LivePreviewProtocol, png []byte, canvasWidth int) (int, error) {
	const (
		leftPanelWidth  = 30
		propertiesWidth = 28
		gap             = 2
		bodyTop         = 3
	)
	panelLeft := leftPanelWidth + gap + canvasWidth + gap + 1
	cols, rows := livePreviewPanelCellSize(propertiesWidth)
	left := panelLeft
	top := bodyTop + 2
	escape := terminalImageEscape(protocol, png, cols, rows)
	if escape == "" {
		return 0, nil
	}
	return fmt.Fprintf(w, "\x1b7%s%s\x1b[%d;%dH%s\x1b8", terminalLivePreviewDeleteEscape(), clearTerminalLivePreview(left, top, propertiesWidth, rows), top, left, escape)
}

func writeTerminalLivePreviewClear(w io.Writer, canvasWidth int) (int, error) {
	const (
		leftPanelWidth  = 30
		propertiesWidth = 28
		gap             = 2
		bodyTop         = 3
	)
	panelLeft := leftPanelWidth + gap + canvasWidth + gap + 1
	_, rows := livePreviewPanelCellSize(propertiesWidth)
	left := panelLeft
	top := bodyTop + 2
	return fmt.Fprintf(w, "\x1b7%s%s\x1b8", terminalLivePreviewDeleteEscape(), clearTerminalLivePreview(left, top, propertiesWidth, rows))
}

func terminalLivePreviewDeleteEscape() string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d;\x1b\\", terminalLivePreviewImageID)
}

func clearTerminalLivePreview(left, top, width, rows int) string {
	var b strings.Builder
	blank := strings.Repeat(" ", max(width, 1))
	for row := 0; row < rows; row++ {
		fmt.Fprintf(&b, "\x1b[%d;%dH%s", top+row, left, blank)
	}
	return b.String()
}

func (m Model) livePreviewKey() string {
	if m.Preview.Protocol == LivePreviewDisabled {
		return ""
	}
	return documentPreviewKey(m.Document, m.Print)
}

func documentPreviewKey(doc label.Document, printConfig PrintConfig) string {
	payload := struct {
		Document  label.Document `json:"document"`
		Model     string         `json:"model"`
		OffsetXMM float64        `json:"offset_x_mm"`
		OffsetYMM float64        `json:"offset_y_mm"`
	}{
		Document:  doc,
		Model:     printConfig.Model,
		OffsetXMM: printConfig.OffsetXMM,
		OffsetYMM: printConfig.OffsetYMM,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("%#v", payload)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func pngHash(png []byte) string {
	sum := sha256.Sum256(png)
	return hex.EncodeToString(sum[:])
}

func terminalImageEscape(protocol LivePreviewProtocol, png []byte, cols, rows int) string {
	if len(png) == 0 || cols <= 0 || rows <= 0 {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(png)
	switch protocol {
	case LivePreviewKitty:
		return kittyImageEscape(encoded, cols, rows)
	default:
		return ""
	}
}

func livePreviewPanelCellSize(width int) (int, int) {
	cols := min(max(width-4, 10), 20)
	rows := 4
	cols = min(cols, max(width, 1))
	return cols, rows
}

func kittyImageEscape(encoded string, cols, rows int) string {
	const chunkSize = 4096
	var b strings.Builder
	for start := 0; start < len(encoded); start += chunkSize {
		end := start + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		more := 0
		if end < len(encoded) {
			more = 1
		}
		if start == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,i=4242,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, encoded[start:end])
			continue
		}
		fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, encoded[start:end])
	}
	return b.String()
}

func (m Model) hasTerminalLivePreview() bool {
	return m.Preview.Protocol == LivePreviewKitty && len(m.Preview.PNG) > 0
}
