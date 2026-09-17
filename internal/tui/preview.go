package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/render"
)

const previewOutputPath = "testlabels/preview.png"

type printResultMsg struct {
	OK       bool
	Printer  string
	Copies   int
	Err      error
	WidthPx  int
	HeightPx int
	Closed   bool
}

func (m *Model) exportPreview() bool {
	result, err := render.RenderDocument(m.Document)
	if err != nil {
		m.setStatus("Preview render failed: %v", err)
		return true
	}
	result, err = m.preparePrintPreview(result)
	if err != nil {
		m.setStatus("Preview print fitting failed: %v", err)
		return true
	}
	if err := os.MkdirAll("testlabels", 0o755); err != nil {
		m.setStatus("Preview directory failed: %v", err)
		return true
	}
	if err := os.WriteFile(previewOutputPath, result.PreviewPNG, 0o644); err != nil {
		m.setStatus("Preview write failed: %v", err)
		return true
	}
	opened, err := openPreviewFile(previewOutputPath)
	if err != nil {
		m.setStatus("Preview written to %s (%dx%d). Open failed: %v", previewOutputPath, result.WidthPx, result.HeightPx, err)
		return true
	}
	if opened {
		m.setStatus("Preview opened from %s (%dx%d).", previewOutputPath, result.WidthPx, result.HeightPx)
		return true
	}
	m.setStatus("Preview written to %s (%dx%d).", previewOutputPath, result.WidthPx, result.HeightPx)
	return true
}

func (m Model) preparePrintPreview(result render.Result) (render.Result, error) {
	if m.Print.Model == "" {
		return result, nil
	}
	fitted, err := render.FitToPrinterWidth(result, m.Print.Model)
	if err != nil {
		return render.Result{}, err
	}
	offsetX, offsetY := render.ModelPrintOffsetMM(m.Print.Model, m.Print.OffsetXMM, m.Print.OffsetYMM)
	return render.ApplyPrintOffset(fitted, offsetX, offsetY)
}

func openPreviewFile(path string) (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return true, exec.Command("open", path).Start()
	default:
		return false, nil
	}
}

func (m *Model) printCurrentDocument() tea.Cmd {
	if m.Print.Session == nil {
		m.setStatus("Printing unavailable. Run setup or pass --config/--printer.")
		return nil
	}
	if m.Connection == ConnectionConnecting {
		m.setStatus("Printer still connecting.")
		return nil
	}
	if m.Connection != ConnectionConnected {
		m.setStatus("Printer disconnected. Press c to reconnect.")
		return nil
	}
	m.setStatus("Printing current label...")
	doc := m.Document
	copies := m.Print.Copies
	session := m.Print.Session
	return func() tea.Msg {
		result, err := render.RenderDocument(doc)
		if err != nil {
			return printResultMsg{Err: fmt.Errorf("render label: %w", err)}
		}
		resp := session.PrintImage(context.Background(), result, copies)
		if !resp.OK {
			if resp.Error != nil {
				return printResultMsg{Err: fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message), WidthPx: result.WidthPx, HeightPx: result.HeightPx}
			}
			return printResultMsg{Err: fmt.Errorf("print failed"), WidthPx: result.WidthPx, HeightPx: result.HeightPx}
		}
		return printResultMsg{OK: true, Printer: resp.Printer, Copies: resp.Copies, WidthPx: result.WidthPx, HeightPx: result.HeightPx, Closed: true}
	}
}

func (m *Model) handlePrintResult(msg printResultMsg) {
	if msg.Err != nil {
		m.setStatus("Print failed: %v", msg.Err)
		return
	}
	if msg.Closed {
		m.Connection = ConnectionDisconnected
		m.ConnectMeta = nil
		m.setStatus("Printed %d copy to %s (%dx%d). Press c to reconnect.", msg.Copies, msg.Printer, msg.WidthPx, msg.HeightPx)
		return
	}
	m.setStatus("Printed %d copy to %s (%dx%d).", msg.Copies, msg.Printer, msg.WidthPx, msg.HeightPx)
}
