package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.EditingText {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.Width = msg.Width
			m.Height = msg.Height
			m.Ready = true
			m.reflow()
			m.refreshStatus()
			return m, nil
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
		return m, nil
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		if msg.String() == "P" {
			return m, m.printCurrentDocument()
		}
		if msg.String() == "c" {
			return m, m.reconnectPrinter()
		}
		if msg.String() == "m" {
			return m, m.enableMouseEditing()
		}
		if m.handleCommandKey(msg) {
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
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

func (m *Model) enableMouseEditing() tea.Cmd {
	if m.MouseEnabled {
		m.setStatus("Mouse editing already enabled.")
		return nil
	}
	m.MouseEnabled = true
	m.setStatus("Mouse editing enabled.")
	return tea.EnableMouseCellMotion
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
	case "delete", "backspace":
		return m.deleteSelected()
	case "k":
		return m.nudgeSelected(0, -1)
	case "j":
		return m.nudgeSelected(0, 1)
	case "h":
		return m.nudgeSelected(-1, 0)
	case "l":
		return m.nudgeSelected(1, 0)
	case "shift+up":
		return m.nudgeSelected(0, -5)
	case "shift+down":
		return m.nudgeSelected(0, 5)
	case "shift+left":
		return m.nudgeSelected(-5, 0)
	case "shift+right":
		return m.nudgeSelected(5, 0)
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
