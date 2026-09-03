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
		if m.handleCommandKey(msg) {
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.Drag = DragState{}
			m.SelectedID = ""
			m.setStatus("Selection cleared.")
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) reflow() {
	toolWidth := 18
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
	case "enter":
		return m.beginEditingSelected()
	case "delete", "backspace":
		return m.deleteSelected()
	case "up":
		return m.nudgeSelected(0, -1)
	case "down":
		return m.nudgeSelected(0, 1)
	case "left":
		return m.nudgeSelected(-1, 0)
	case "right":
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
	default:
		return false
	}
}
