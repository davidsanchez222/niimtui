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
	if scan, ok := msg.(printerScanMsg); ok {
		active := scan.Seq == m.ScanSeq && m.Discovering
		cmd := m.handlePrinterScan(scan)
		if active && m.Prompt.Mode != PromptNone {
			m.refreshPromptStatus()
		}
		if active && m.Tab == tabGallery {
			cmd = tea.Batch(cmd, m.requestGalleryPreview())
		}
		return m, cmd
	}
	if rendered, ok := msg.(galleryRenderedMsg); ok {
		return m, m.onGalleryRendered(rendered)
	}
	if connected, ok := msg.(printerConnectedMsg); ok {
		if connected.Seq != m.ConnectSeq {
			return m, nil
		}
		m.Connection = ConnectionConnected
		m.ConnectErr = ""
		m.ConnectMeta = connected.Info.Meta
		m.setStatus("Printer connected.")
		if m.Prompt.Mode != PromptNone {
			m.refreshPromptStatus()
		}
		return m, nil
	}
	if failed, ok := msg.(printerConnectionFailedMsg); ok {
		if failed.Seq != m.ConnectSeq {
			return m, nil
		}
		m.Connection = ConnectionDisconnected
		m.ConnectErr = failed.Err.Error()
		m.setStatus("Printer connection failed: %v", failed.Err)
		if m.Prompt.Mode != PromptNone {
			m.refreshPromptStatus()
		}
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		m.closePrinterSession()
		return m, tea.Quit
	}
	if resized, ok := msg.(tea.WindowSizeMsg); ok {
		m.Selection = textSelection{}
		m.Width = resized.Width
		m.Height = resized.Height
		m.Ready = true
		m.reflow()
		m.refreshStatus()
		return m, m.livePreviewResizeCmd()
	}
	if mouse, ok := msg.(tea.MouseMsg); ok {
		if handled, cmd := m.handleSelectableMouse(mouse); handled {
			return m, cmd
		}
	}
	if _, ok := msg.(tea.KeyMsg); ok {
		m.Selection = textSelection{}
	}
	if m.Prompt.Mode != PromptNone {
		switch input := msg.(type) {
		case tea.KeyMsg:
			wasOpen := m.HelpOpen || m.MenuOpen || m.confirmPromptOpen()
			m.handlePromptKey(input)
			return m, m.livePreviewModalCmd(wasOpen)
		case tea.MouseMsg:
			return m, nil
		}
	}
	if m.EditingText {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			m.handleTextEditing(msg)
			return m, nil
		case tea.MouseMsg:
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.MenuOpen || m.HelpOpen {
			return m, nil
		}
		if m.Tab == tabGallery {
			return m, nil
		}
		updated, cmd := m.updateMouse(msg)
		if model, ok := updated.(Model); ok {
			return model, cmd
		}
		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "1" && !m.MenuOpen && !m.HelpOpen {
			return m, m.switchTab(tabDesigner)
		}
		if msg.String() == "2" && !m.MenuOpen && !m.HelpOpen {
			return m, m.switchTab(tabGallery)
		}
		if msg.String() == "3" {
			wasOpen := m.HelpOpen || m.MenuOpen
			m.toggleMenu()
			return m, m.livePreviewModalCmd(wasOpen)
		}
		if msg.String() == "?" {
			wasOpen := m.HelpOpen || m.MenuOpen
			m.HelpOpen = !m.HelpOpen
			if m.HelpOpen {
				m.MenuOpen = false
				m.FocusPickerOpen = false
				m.closeFontPicker()
				m.setStatus("Help opened. Press ? or esc to close.")
			} else {
				m.setStatus("Help closed.")
			}
			return m, m.livePreviewModalCmd(wasOpen)
		}
		if msg.String() == "ctrl+t" && !m.MenuOpen && !m.HelpOpen {
			if m.Tab == tabGallery {
				return m, m.switchTab(tabDesigner)
			}
			return m, m.switchTab(tabGallery)
		}
		if m.MenuOpen {
			if msg.String() == "q" {
				m.closePrinterSession()
				return m, tea.Quit
			}
			wasOpen := m.HelpOpen || m.MenuOpen
			cmd := m.handleMenuKey(msg)
			return m, tea.Batch(cmd, m.livePreviewModalCmd(wasOpen))
		}
		if m.HelpOpen {
			wasOpen := m.HelpOpen || m.MenuOpen
			switch msg.String() {
			case "q":
				m.closePrinterSession()
				return m, tea.Quit
			case "?", "esc":
				m.HelpOpen = false
				m.setStatus("Help closed.")
			}
			return m, m.livePreviewModalCmd(wasOpen)
		}
		if m.Tab == tabGallery {
			return m, m.handleGalleryKey(msg)
		}
		if m.SidebarFocused {
			return m, m.handleSidebarKey(msg)
		}
		if msg.String() == "tab" || msg.String() == "shift+tab" {
			m.FocusPickerOpen = false
			m.closeFontPicker()
			m.focusSidebar()
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
		if msg.String() == "z" {
			m.undo()
			return m, nil
		}
		if msg.String() == "Z" {
			m.redo()
			return m, nil
		}
		if msg.String() == "B" {
			m.cycleRedoBranch()
			return m, nil
		}
		if m.handleCommandKey(msg) {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			m.Drag = DragState{}
			m.SelectedID = ""
			m.setStatus("Selection cleared.")
			return m, nil
		}
	case printResultMsg:
		cmd := m.handlePrintResult(msg)
		if m.Prompt.Mode != PromptNone {
			m.refreshPromptStatus()
		}
		return m, cmd
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
			if m.Prompt.Mode != PromptNone {
				m.refreshPromptStatus()
			}
		}
		return m, nil
	case livePreviewRedrawMsg:
		if m.hasTerminalLivePreview() && !m.HelpOpen && !m.MenuOpen && !m.confirmPromptOpen() && msg.Seq == m.Preview.RedrawSeq {
			if m.Tab == tabGallery {
				return m, galleryTerminalPreviewCmd(m)
			}
			return m, terminalLivePreviewCmd(m)
		}
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
	if m.Print.Session == nil && m.Print.NewSession == nil {
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
	if m.Discovering {
		if m.scanCancel != nil {
			m.scanCancel()
		}
		m.PendingConnect = true
		m.setStatus("Stopping discovery before connecting to %s...", m.Print.Printer)
		return nil
	}
	return m.connectSelectedPrinter()
}

func (m *Model) disconnectPrinter() {
	if m.Print.Session == nil || m.Connection != ConnectionConnected {
		m.setStatus("Printer is not connected.")
		return
	}
	err := m.Print.Session.Close()
	m.ConnectSeq++
	m.Connection = ConnectionDisconnected
	m.ConnectMeta = nil
	m.ConnectErr = ""
	if err != nil {
		m.ConnectErr = err.Error()
		m.setStatus("Printer disconnected with error: %v", err)
		return
	}
	m.setStatus("Printer disconnected. Press c to reconnect.")
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
	case "q":
		m.addQRElement()
		return true
	case "r":
		return m.rotateSelectedElement()
	case "R":
		return m.rotateCanvas()
	case "i":
		return m.beginEditingSelected()
	case "b":
		return m.beginBindingPrompt()
	case "!":
		return m.toggleSelectedBindingRequired()
	case "y":
		return m.copySelected()
	case "x":
		return m.cutSelected()
	case "v":
		return m.pasteClipboard()
	case "d":
		return m.duplicateSelected()
	case "f":
		return m.openFocusPicker()
	case "F":
		return m.toggleFontPicker()
	case "e":
		return m.beginExportPNGPrompt()
	case "g":
		return m.toggleGrid()
	case "I":
		return m.toggleInvertedColors()
	case "s":
		m.beginSaveDesignPrompt()
		return true
	case "m":
		return m.toggleMenu()
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
		return m.nudgeSelectedCells(0, -1)
	case "j", "down":
		return m.nudgeSelectedCells(0, 1)
	case "h", "left":
		return m.nudgeSelectedCells(-1, 0)
	case "l", "right":
		return m.nudgeSelectedCells(1, 0)
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
