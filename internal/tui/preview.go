package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/config"
	"niimtui/internal/printtrace"
	"niimtui/internal/render"
)

const previewOutputPath = "testlabels/preview.png"

type printResultMsg struct {
	OK           bool
	Printer      string
	Copies       int
	Err          error
	Disconnected bool
	WidthPx      int
	HeightPx     int
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
	return preparePrintPreviewResult(result, m.Print)
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
		m.setStatus("Printer not connected. Press Tab to focus printers, then c to connect.")
		return nil
	}
	if m.Print.Model != "" && (m.Preset < 0 || m.Preset >= len(m.Presets) || !config.MatchesStock(m.Document, m.Presets[m.Preset])) {
		m.setStatus("Design size/shape does not match the selected label roll. Select a matching roll before printing.")
		return nil
	}
	if m.Connection == ConnectionConnecting {
		m.setStatus("Printer still connecting.")
		return nil
	}
	if m.Connection != ConnectionConnected {
		m.setStatus("Printer disconnected. Press Tab to focus printers, then c to reconnect.")
		return nil
	}
	m.setStatus("Printing current label...")
	ctx := printtrace.Start(context.Background())
	printtrace.Mark(ctx, "TUI print requested")
	doc := m.Document
	copies := m.Print.Copies
	session := m.Print.Session
	return func() tea.Msg {
		result, err := render.RenderDocument(doc)
		if err != nil {
			return printResultMsg{Err: fmt.Errorf("render label: %w", err)}
		}
		printtrace.Mark(ctx, "render complete")
		resp := session.PrintImage(ctx, result, copies)
		if !resp.OK {
			if resp.Error != nil {
				return printResultMsg{Err: fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message), Disconnected: session.Metadata() == nil, WidthPx: result.WidthPx, HeightPx: result.HeightPx}
			}
			return printResultMsg{Err: fmt.Errorf("print failed"), Disconnected: session.Metadata() == nil, WidthPx: result.WidthPx, HeightPx: result.HeightPx}
		}
		return printResultMsg{OK: true, Printer: resp.Printer, Copies: resp.Copies, WidthPx: result.WidthPx, HeightPx: result.HeightPx}
	}
}

func (m *Model) handlePrintResult(msg printResultMsg) tea.Cmd {
	if msg.Err != nil {
		if msg.Disconnected {
			m.Connection = ConnectionDisconnected
			m.ConnectMeta = nil
		}
		m.setStatus("Print failed: %v", msg.Err)
		return nil
	}
	m.setStatus("Printed %d copy to %s (%dx%d).", msg.Copies, msg.Printer, msg.WidthPx, msg.HeightPx)
	return nil
}
