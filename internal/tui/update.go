package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	beforePreviewKey := m.livePreviewKey()
	updated, cmd := m.update(msg)
	return updated.withLivePreviewSchedule(beforePreviewKey, cmd)
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
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
		case "q", "ctrl+c":
			m.closePrinterSession()
			return m, tea.Quit
		case "esc":
			if m.HelpOpen {
				m.HelpOpen = false
				m.setStatus("Help closed.")
				return m, nil
			}
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
	toolWidth := 30
	propertiesWidth := 28
	framePadding := 8

	canvasWidth := max(m.Width-toolWidth-propertiesWidth-framePadding, 12)
	canvasHeight := max(m.Height-6, 6)
	m.Canvas = newCanvas(canvasWidth, canvasHeight, m.Document.WidthMM, m.Document.HeightMM)
	m.Canvas.X = toolWidth + 2
	m.Canvas.Y = 2
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
	case "?":
		m.HelpOpen = !m.HelpOpen
		if m.HelpOpen {
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
		return m.resizeSelected(HandleLeft, -1, 0)
	case "J":
		return m.resizeSelected(HandleBottom, 0, 1)
	case "K":
		return m.resizeSelected(HandleTop, 0, -1)
	case "L":
		return m.resizeSelected(HandleRight, 1, 0)
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
