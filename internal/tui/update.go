package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	beforePreviewKey := m.livePreviewKey()
	updated, cmd := m.update(msg)
	return updated.withLivePreviewSchedule(beforePreviewKey, cmd)
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		m.closePrinterSession()
		return m, tea.Quit
	}
	if m.EditingText {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.Width = msg.Width
			m.Height = msg.Height
			m.Ready = true
			m.reflow()
			m.refreshStatus()
			return m, m.livePreviewResizeCmd()
		case tea.KeyMsg:
			m.handleTextEditing(msg)
			return m, nil
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true
		m.reflow()
		m.refreshStatus()
		return m, m.livePreviewResizeCmd()
	case tea.MouseMsg:
		updated, cmd := m.updateMouse(msg)
		if model, ok := updated.(Model); ok {
			return model, cmd
		}
		return m, cmd
	case tea.KeyMsg:
		if m.MenuOpen {
			m.handleMenuKey(msg)
			return m, nil
		}
		if m.HelpOpen {
			switch msg.String() {
			case "?", "esc":
				m.HelpOpen = false
				m.setStatus("Help closed.")
			}
			return m, nil
		}
		if m.FocusPickerOpen {
			m.handleFocusPickerKey(msg)
			return m, nil
		}
		if m.FontPickerOpen && m.handleFontPickerKey(msg) {
			return m, nil
		}
		if msg.String() == "P" {
			return m, m.printCurrentDocument()
		}
		if msg.String() == "c" {
			return m, m.reconnectPrinter()
		}
		if m.handleCommandKey(msg) {
			return m, nil
		}
		switch msg.String() {
		case "q":
			m.closePrinterSession()
			return m, tea.Quit
		case "esc":
			m.Drag = DragState{}
			m.SelectedID = ""
			m.setStatus("Selection cleared.")
			return m, nil
		}
	case printResultMsg:
		m.handlePrintResult(msg)
		return m, nil
	case livePreviewTickMsg:
		if msg.Seq != m.Preview.RequestedSeq || m.Preview.Protocol == LivePreviewDisabled {
			return m, nil
		}
		return m, renderLivePreviewCmd(m, msg.Seq)
	case livePreviewRenderedMsg:
		return m.handleLivePreviewRendered(msg)
	case livePreviewFailedMsg:
		if msg.Seq == m.Preview.RequestedSeq {
			m.Preview.Err = msg.Err.Error()
			m.setStatus("Realtime preview failed: %v", msg.Err)
		}
		return m, nil
	case livePreviewRedrawMsg:
		if m.hasTerminalLivePreview() && msg.Seq == m.Preview.RedrawSeq {
			return m, terminalLivePreviewCmd(m)
		}
		return m, nil
	case printerConnectedMsg:
		m.Connection = ConnectionConnected
		m.ConnectErr = ""
		m.ConnectMeta = msg.Info.Meta
		m.setStatus("Printer connected.")
		return m, nil
	case printerConnectionFailedMsg:
		m.Connection = ConnectionDisconnected
		m.ConnectErr = msg.Err.Error()
		m.setStatus("Printer connection failed: %v", msg.Err)
		return m, nil
	}

	return m, nil
}

func (m *Model) livePreviewResizeCmd() tea.Cmd {
	if m.hasTerminalLivePreview() {
		if m.isTerminalTooSmall() {
			return clearTerminalLivePreviewCmd(m.canvasPanelWidth())
		}
		m.Preview.RedrawSeq++
		return livePreviewRedrawCmd(m.Preview.RedrawSeq)
	}
	return nil
}

func (m Model) withLivePreviewSchedule(beforeKey string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if m.Preview.Protocol == LivePreviewDisabled {
		return m, cmd
	}
	afterKey := m.livePreviewKey()
	if beforeKey == afterKey {
		return m, cmd
	}
	m.Preview.RequestedSeq++
	m.Preview.LastKey = afterKey
	return m, tea.Batch(cmd, livePreviewDebounceCmd(m.Preview.RequestedSeq))
}

func (m *Model) reconnectPrinter() tea.Cmd {
	if m.Print.Session == nil {
		m.setStatus("Printer connection unavailable. Run setup or pass --config/--printer.")
		return nil
	}
	if m.Connection == ConnectionConnected {
		m.setStatus("Printer already connected.")
		return nil
	}
	if m.Connection == ConnectionConnecting {
		m.setStatus("Printer already connecting.")
		return nil
	}
	m.Connection = ConnectionConnecting
	m.ConnectErr = ""
	m.setStatus("Connecting to printer...")
	return connectPrinterCmd(m.Print.Session)
}

func (m *Model) closePrinterSession() {
	if m.Print.Session == nil {
		return
	}
	_ = m.Print.Session.Close()
}

func (m *Model) reflow() {
	canvasWidth := m.canvasPanelWidth()
	canvasHeight := m.canvasPanelHeight()
	m.Canvas = newCanvas(canvasWidth, canvasHeight, m.Document.WidthMM, m.Document.HeightMM)
	m.Canvas.X = m.canvasPanelLeft() + max((canvasWidth-m.Canvas.Width)/2, 0)
	m.Canvas.Y = layoutBodyTop + max((canvasHeight-m.Canvas.Height)/2, 0)
}

func (m *Model) handleCommandKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "t":
		m.addTextElement()
		return true
	case "r":
		m.addQRElement()
		return true
	case "i":
		return m.beginEditingSelected()
	case "f":
		return m.openFocusPicker()
	case "F":
		return m.toggleFontPicker()
	case "m":
		return m.toggleMenu()
	case "n":
		return m.switchPreset(1)
	case "N":
		return m.switchPreset(-1)
	case "?":
		m.HelpOpen = !m.HelpOpen
		if m.HelpOpen {
			m.MenuOpen = false
			m.FocusPickerOpen = false
			m.closeFontPicker()
			m.setStatus("Help opened. Press ? or esc to close.")
		} else {
			m.setStatus("Help closed.")
		}
		return true
	case "delete", "backspace":
		return m.deleteSelected()
	case "k", "up":
		return m.nudgeSelected(0, -1)
	case "j", "down":
		return m.nudgeSelected(0, 1)
	case "h", "left":
		return m.nudgeSelected(-1, 0)
	case "l", "right":
		return m.nudgeSelected(1, 0)
	case "H":
		return m.resizeSelectedDimensions(-1, 0)
	case "J":
		return m.resizeSelectedDimensions(0, 1)
	case "K":
		return m.resizeSelectedDimensions(0, -1)
	case "L":
		return m.resizeSelectedDimensions(1, 0)
	case "]":
		return m.resizeSelected(HandleBottomRight, 1, 1)
	case "[":
		return m.resizeSelected(HandleBottomRight, -1, -1)
	case "}":
		return m.resizeSelected(HandleBottomRight, 5, 5)
	case "{":
		return m.resizeSelected(HandleBottomRight, -5, -5)
	case "+", "=":
		return m.adjustSelectedFont(1)
	case "-":
		return m.adjustSelectedFont(-1)
	case "p":
		return m.exportPreview()
	default:
		return false
	}
}

func (m *Model) switchPreset(delta int) bool {
	if len(m.Presets) == 0 || delta == 0 {
		m.setStatus("No label presets available.")
		return true
	}
	index := m.Preset
	if index < 0 || index >= len(m.Presets) {
		index = activePresetIndex(m.Presets, "", m.Document)
	}
	if index < 0 {
		index = 0
	} else {
		index = (index + delta) % len(m.Presets)
		if index < 0 {
			index += len(m.Presets)
		}
	}
	m.applyPreset(index)
	return true
}

func (m *Model) applyPreset(index int) {
	if index < 0 || index >= len(m.Presets) {
		return
	}
	preset := m.Presets[index]
	m.Preset = index
	m.Document.WidthMM = preset.WidthMM
	m.Document.HeightMM = preset.HeightMM
	m.Document.Shape = strings.ToLower(strings.TrimSpace(preset.Shape))
	if m.Document.Shape == "" {
		m.Document.Shape = "rect"
	}
	m.SelectedID = ""
	m.Drag = DragState{}
	m.reflow()
	m.setStatus("Label preset: %s (%.0fx%.0f %s).", preset.Name, preset.WidthMM, preset.HeightMM, m.Document.Shape)
}
