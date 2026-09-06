package tui

import (
	"math"

	tea "github.com/charmbracelet/bubbletea"

	"niimcli/internal/label"
	"niimcli/internal/render"
)

const (
	minElementWidthMM  = 6.0
	minElementHeightMM = 4.0
)

type screenRect struct {
	left   int
	top    int
	right  int
	bottom int
}

type handlePoint struct {
	handle ResizeHandle
	x      int
	y      int
}

func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		m.handleMousePress(msg)
	case tea.MouseActionMotion:
		if m.Drag.Mode == DragNone {
			return m, nil
		}
		m.handleMouseMotion(msg)
	case tea.MouseActionRelease:
		if m.Drag.Mode != DragNone {
			m.setStatus("Drag complete.")
		}
		m.Drag = DragState{}
	}

	return m, nil
}

func (m *Model) handleMousePress(msg tea.MouseMsg) {
	if element, ok := m.selectedElement(); ok {
		if handle, ok := m.handleAt(element, msg.X, msg.Y); ok {
			m.Drag = DragState{
				Mode:            DragResize,
				Handle:          handle,
				StartMouseX:     msg.X,
				StartMouseY:     msg.Y,
				OriginalElement: element,
			}
			m.setStatus("Resizing selected element.")
			return
		}
		if m.hitElement(element, msg.X, msg.Y) {
			m.Drag = DragState{
				Mode:            DragMove,
				StartMouseX:     msg.X,
				StartMouseY:     msg.Y,
				OriginalElement: element,
			}
			m.setStatus("Moving selected element.")
			return
		}
	}

	if element, ok := m.elementAt(msg.X, msg.Y); ok {
		m.SelectedID = element.ID
		m.setStatus("Selected %q.", selectedLabel(element))
		return
	}

	m.SelectedID = ""
	m.setStatus("No selection.")
}

func (m *Model) handleMouseMotion(msg tea.MouseMsg) {
	if m.SelectedID == "" {
		m.Drag = DragState{}
		return
	}

	dxCells := msg.X - m.Drag.StartMouseX
	dyCells := msg.Y - m.Drag.StartMouseY
	dxMM, dyMM := m.Canvas.CellsToMM(dxCells, dyCells)

	updated := m.Drag.OriginalElement
	switch m.Drag.Mode {
	case DragMove:
		updated.XMM = m.Drag.OriginalElement.XMM + dxMM
		updated.YMM = m.Drag.OriginalElement.YMM + dyMM
	case DragResize:
		updated = resizeElement(m.Drag.OriginalElement, m.Drag.Handle, dxMM, dyMM)
	}

	clampElementToDocument(&updated, m.Document)
	m.Document.UpdateElement(updated)
	if m.Drag.Mode == DragMove {
		m.setStatus("Moving: x %.1fmm y %.1fmm", updated.XMM, updated.YMM)
	} else {
		m.setStatus("Resizing: %.1fmm x %.1fmm", updated.WidthMM, updated.HeightMM)
	}
}

func (m Model) elementAt(x, y int) (label.Element, bool) {
	for i := len(m.Document.Elements) - 1; i >= 0; i-- {
		element := m.Document.Elements[i]
		if m.hitElement(element, x, y) {
			return element, true
		}
	}
	return label.Element{}, false
}

func (m Model) hitElement(element label.Element, x, y int) bool {
	r := m.elementScreenRect(element)
	return x >= r.left && x <= r.right && y >= r.top && y <= r.bottom
}

func (m Model) handleAt(element label.Element, x, y int) (ResizeHandle, bool) {
	const handleHitRadius = 1
	for _, point := range m.handlePoints(element) {
		if absInt(x-point.x) <= handleHitRadius && absInt(y-point.y) <= handleHitRadius {
			return point.handle, true
		}
	}
	return HandleNone, false
}

func (m Model) elementScreenRect(element label.Element) screenRect {
	left, top := m.Canvas.LabelToScreen(element.XMM, element.YMM)
	right, bottom := m.Canvas.LabelToScreen(element.XMM+element.WidthMM, element.YMM+element.HeightMM)

	maxX := m.Canvas.X + m.Canvas.Width - 2
	maxY := m.Canvas.Y + m.Canvas.Height - 2

	left = clampInt(left, m.Canvas.X+1, maxX)
	top = clampInt(top, m.Canvas.Y+1, maxY)
	right = clampInt(right, left, maxX)
	bottom = clampInt(bottom, top, maxY)

	return screenRect{left: left, top: top, right: right, bottom: bottom}
}

func (m Model) handlePoints(element label.Element) []handlePoint {
	r := m.elementScreenRect(element)
	midX := r.left + (r.right-r.left)/2
	midY := r.top + (r.bottom-r.top)/2
	return []handlePoint{
		{handle: HandleTopLeft, x: r.left, y: r.top},
		{handle: HandleTop, x: midX, y: r.top},
		{handle: HandleTopRight, x: r.right, y: r.top},
		{handle: HandleLeft, x: r.left, y: midY},
		{handle: HandleRight, x: r.right, y: midY},
		{handle: HandleBottomLeft, x: r.left, y: r.bottom},
		{handle: HandleBottom, x: midX, y: r.bottom},
		{handle: HandleBottomRight, x: r.right, y: r.bottom},
	}
}

func (m Model) selectedElement() (label.Element, bool) {
	if m.SelectedID == "" {
		return label.Element{}, false
	}
	return m.Document.ElementByID(m.SelectedID)
}

func clampElementToDocument(element *label.Element, doc label.Document) bool {
	if element == nil {
		return false
	}
	changed := false

	minWidthMM, minHeightMM := minimumElementSize(*element)
	if element.WidthMM < minWidthMM {
		element.WidthMM = minWidthMM
		changed = true
	}
	if element.HeightMM < minHeightMM {
		element.HeightMM = minHeightMM
		changed = true
	}
	if element.WidthMM > doc.WidthMM {
		element.WidthMM = doc.WidthMM
		changed = true
	}
	if element.HeightMM > doc.HeightMM {
		element.HeightMM = doc.HeightMM
		changed = true
	}
	if element.XMM < 0 {
		element.XMM = 0
		changed = true
	}
	if element.YMM < 0 {
		element.YMM = 0
		changed = true
	}
	if element.XMM+element.WidthMM > doc.WidthMM {
		element.XMM = math.Max(0, doc.WidthMM-element.WidthMM)
		changed = true
	}
	if element.YMM+element.HeightMM > doc.HeightMM {
		element.YMM = math.Max(0, doc.HeightMM-element.HeightMM)
		changed = true
	}

	return changed
}

func clampInt(v, minValue, maxValue int) int {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func resizeElement(original label.Element, handle ResizeHandle, dxMM, dyMM float64) label.Element {
	updated := original
	left := original.XMM
	top := original.YMM
	right := original.XMM + original.WidthMM
	bottom := original.YMM + original.HeightMM

	switch handle {
	case HandleTopLeft:
		left += dxMM
		top += dyMM
	case HandleTop:
		top += dyMM
	case HandleTopRight:
		right += dxMM
		top += dyMM
	case HandleLeft:
		left += dxMM
	case HandleRight:
		right += dxMM
	case HandleBottomLeft:
		left += dxMM
		bottom += dyMM
	case HandleBottom:
		bottom += dyMM
	case HandleBottomRight:
		right += dxMM
		bottom += dyMM
	}

	proposedWidth := right - left
	minWidthMM, _ := minimumElementSize(updated)
	if proposedWidth < minWidthMM {
		switch handle {
		case HandleTopLeft, HandleLeft, HandleBottomLeft:
			left = right - minWidthMM
		default:
			right = left + minWidthMM
		}
	}
	updated.XMM = left
	updated.YMM = top
	updated.WidthMM = math.Max(minWidthMM, right-left)
	updated.HeightMM = math.Max(minElementHeightMM, bottom-top)
	_, minHeightMM := minimumElementSize(updated)
	if bottom-top < minHeightMM {
		switch handle {
		case HandleTopLeft, HandleTop, HandleTopRight:
			top = bottom - minHeightMM
		default:
			bottom = top + minHeightMM
		}
	}

	updated.XMM = left
	updated.YMM = top
	updated.WidthMM = math.Max(minWidthMM, right-left)
	updated.HeightMM = math.Max(minHeightMM, bottom-top)
	return updated
}

func minimumElementSize(element label.Element) (float64, float64) {
	minWidthMM := minElementWidthMM
	minHeightMM := minElementHeightMM
	if element.Text == nil {
		return minWidthMM, minHeightMM
	}
	renderMinWidthMM, err := render.MinimumTextWidthMM(element)
	if err == nil && renderMinWidthMM > minWidthMM {
		minWidthMM = renderMinWidthMM
	}
	requiredHeightMM, err := render.RequiredTextHeightMM(element, math.Max(element.WidthMM, minWidthMM))
	if err == nil && requiredHeightMM > minHeightMM {
		minHeightMM = requiredHeightMM
	}
	return minWidthMM, minHeightMM
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func selectedLabel(element label.Element) string {
	if element.Text == nil || element.Text.Value == "" {
		return element.ID
	}
	return element.Text.Value
}
