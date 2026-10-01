package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type selectionPoint struct{ X, Y int }

type textSelection struct {
	Start  selectionPoint
	End    selectionPoint
	Lines  []string
	Active bool
	Moved  bool
}

func (m Model) canvasMouseTarget(x, y int) bool {
	return m.Tab == tabDesigner && !m.EditingText && !m.MenuOpen && !m.HelpOpen && !m.confirmPromptOpen() &&
		x >= m.Canvas.X && x < m.Canvas.X+m.Canvas.Width && y >= m.Canvas.Y && y < m.Canvas.Y+m.Canvas.Height
}

func (m *Model) handleSelectableMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	if m.Selection.Active {
		switch msg.Action {
		case tea.MouseActionMotion:
			m.Selection.End = selectionPoint{msg.X, msg.Y}
			m.Selection.Moved = m.Selection.End != m.Selection.Start
			return true, nil
		case tea.MouseActionRelease:
			m.Selection.End = selectionPoint{msg.X, msg.Y}
			m.Selection.Moved = m.Selection.End != m.Selection.Start
			start, moved := m.Selection.Start, m.Selection.Moved
			text := selectedText(m.Selection)
			m.Selection.Active = false
			if !moved {
				m.Selection = textSelection{}
				return m.clickTextAt(start.X, start.Y)
			}
			if strings.TrimSpace(text) == "" {
				m.setStatus("No text selected.")
				return true, nil
			}
			if m.Prompt.Mode == PromptCommand && strings.HasPrefix(strings.TrimSpace(text), "niimtui print") {
				text = m.Prompt.Value
			}
			if err := copyTerminalCommand(text); err != nil {
				m.setStatus("Could not copy selected text: %v", err)
			} else {
				m.setStatus("Selected text sent to terminal clipboard (if supported).")
			}
			return true, nil
		}
		return true, nil
	}
	if msg.Action == tea.MouseActionPress {
		m.Selection = textSelection{}
	}
	if !m.Ready || msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft || m.canvasMouseTarget(msg.X, msg.Y) {
		return false, nil
	}
	m.Selection = textSelection{
		Start: selectionPoint{msg.X, msg.Y}, End: selectionPoint{msg.X, msg.Y},
		Lines: strings.Split(ansi.Strip(m.View()), "\n"), Active: true,
	}
	return true, nil
}

func (m *Model) clickTextAt(x, y int) (bool, tea.Cmd) {
	if m.Prompt.Mode != PromptNone || m.MenuOpen || m.HelpOpen || m.EditingText {
		return true, nil
	}
	if y == 1 {
		if tab, ok := tabAt(m.Width, x); ok {
			return true, m.switchTab(tab)
		}
	}
	if m.Tab == tabGallery && y >= layoutBodyTop+3 && x < layoutLeftPanelWidth {
		rows := m.galleryRows()
		start, _ := sidebarWindow(len(rows), m.GalleryIndex, max(1, m.canvasPanelHeight()-7))
		index := start + y - layoutBodyTop - 3
		if index >= 0 && index < len(rows) {
			m.GalleryIndex = index
			if rows[index].Header {
				return true, m.handleGalleryKey(tea.KeyMsg{Type: tea.KeyEnter})
			}
			cmd := m.requestGalleryPreview()
			if m.Preview.Protocol == LivePreviewKitty {
				cmd = tea.Batch(clearTerminalLivePreviewCmd(m.canvasPanelWidth()), cmd)
			}
			return true, cmd
		}
	}
	return true, nil
}

func selectionRange(s textSelection) (selectionPoint, selectionPoint) {
	start, end := s.Start, s.End
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	return start, end
}

func selectedText(s textSelection) string {
	if !s.Active || !s.Moved {
		return ""
	}
	start, end := selectionRange(s)
	var selected []string
	for y := max(0, start.Y); y <= end.Y && y < len(s.Lines); y++ {
		line := s.Lines[y]
		left, right := 0, ansi.StringWidth(line)
		if y == start.Y {
			left = max(0, start.X)
		}
		if y == end.Y {
			right = max(0, end.X)
		}
		selected = append(selected, strings.TrimRight(ansi.Cut(line, min(left, right), right), " "))
	}
	return strings.Join(selected, "\n")
}

func highlightSelection(view string, s textSelection) string {
	if !s.Moved {
		return view
	}
	lines := strings.Split(view, "\n")
	start, end := selectionRange(s)
	for y := max(0, start.Y); y <= end.Y && y < len(lines); y++ {
		plain := ansi.Strip(lines[y])
		if y < len(s.Lines) {
			plain = s.Lines[y]
		}
		width := ansi.StringWidth(plain)
		left, right := 0, width
		if y == start.Y {
			left = min(max(0, start.X), width)
		}
		if y == end.Y {
			right = min(max(0, end.X), width)
		}
		if left >= right {
			continue
		}
		lines[y] = ansi.Cut(plain, 0, left) + "\x1b[7m" + ansi.Cut(plain, left, right) + "\x1b[0m" + ansi.Cut(plain, right, width)
	}
	return strings.Join(lines, "\n")
}
