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
}

func (m *Model) exportPreview() bool {
	result, err := render.RenderDocument(m.Document)
	if err != nil {
		m.setStatus("Preview render failed: %v", err)
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

func openPreviewFile(path string) (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return true, exec.Command("open", path).Start()
	default:
		return false, nil
	}
}

func (m *Model) printCurrentDocument() tea.Cmd {
	if m.Print.Service == nil {
		m.setStatus("Printing unavailable. Run setup or pass --config/--printer.")
		return nil
	}
	m.setStatus("Printing current label...")
	doc := m.Document
	printer := m.Print.Printer
	copies := m.Print.Copies
	printService := m.Print.Service
	return func() tea.Msg {
		result, err := render.RenderDocument(doc)
		if err != nil {
			return printResultMsg{Err: fmt.Errorf("render label: %w", err)}
		}
		resp := printService.PrintImage(context.Background(), printer, result, copies)
		if !resp.OK {
			if resp.Error != nil {
				return printResultMsg{Err: fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message), WidthPx: result.WidthPx, HeightPx: result.HeightPx}
			}
			return printResultMsg{Err: fmt.Errorf("print failed"), WidthPx: result.WidthPx, HeightPx: result.HeightPx}
		}
		return printResultMsg{OK: true, Printer: resp.Printer, Copies: resp.Copies, WidthPx: result.WidthPx, HeightPx: result.HeightPx}
	}
}

func (m *Model) handlePrintResult(msg printResultMsg) {
	if msg.Err != nil {
		m.setStatus("Print failed: %v", msg.Err)
		return
	}
	m.setStatus("Printed %d copy to %s (%dx%d).", msg.Copies, msg.Printer, msg.WidthPx, msg.HeightPx)
}
