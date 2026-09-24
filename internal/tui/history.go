package tui

import (
	"reflect"

	"niimtui/internal/label"
)

func (m *Model) initHistory() {
	m.History = &HistoryNode{Label: "initial", Snapshot: m.snapshot()}
	m.RedoBranch = 0
}

func (m Model) snapshot() HistorySnapshot {
	return HistorySnapshot{
		Document:   cloneDocument(m.Document),
		SelectedID: m.SelectedID,
		NextID:     m.NextID,
		Preset:     m.Preset,
	}
}

func (m *Model) restoreSnapshot(snapshot HistorySnapshot) {
	m.Document = cloneDocument(snapshot.Document)
	m.SelectedID = snapshot.SelectedID
	m.NextID = snapshot.NextID
	m.Preset = snapshot.Preset
	m.Drag = DragState{}
	m.FocusPickerOpen = false
	m.closeFontPicker()
	m.EditingText = false
	m.TextBuffer = ""
	m.reflow()
}

func (m *Model) commitHistory(label string) {
	snapshot := m.snapshot()
	if m.History == nil {
		m.History = &HistoryNode{Label: label, Snapshot: snapshot}
		m.RedoBranch = 0
		return
	}
	if reflect.DeepEqual(m.History.Snapshot, snapshot) {
		return
	}
	child := &HistoryNode{Parent: m.History, Label: label, Snapshot: snapshot}
	m.History.Children = append(m.History.Children, child)
	m.RedoBranch = len(m.History.Children) - 1
	m.History = child
}

func (m *Model) undo() bool {
	if m.History == nil || m.History.Parent == nil {
		m.setStatus("Nothing to undo.")
		return true
	}
	current := m.History
	m.History = current.Parent
	m.RedoBranch = childIndex(m.History, current)
	m.restoreSnapshot(m.History.Snapshot)
	m.setStatus("Undid %s.", current.Label)
	return true
}

func (m *Model) redo() bool {
	if m.History == nil || len(m.History.Children) == 0 {
		m.setStatus("Nothing to redo.")
		return true
	}
	m.RedoBranch = clampInt(m.RedoBranch, 0, len(m.History.Children)-1)
	next := m.History.Children[m.RedoBranch]
	m.History = next
	m.RedoBranch = 0
	m.restoreSnapshot(next.Snapshot)
	m.setStatus("Redid %s.", next.Label)
	return true
}

func (m *Model) cycleRedoBranch() bool {
	if m.History == nil || len(m.History.Children) <= 1 {
		m.setStatus("No alternate redo branches.")
		return true
	}
	m.RedoBranch = (m.RedoBranch + 1) % len(m.History.Children)
	m.setStatus("Redo branch %d/%d selected. Press Z to redo %s.", m.RedoBranch+1, len(m.History.Children), m.History.Children[m.RedoBranch].Label)
	return true
}

func childIndex(parent, child *HistoryNode) int {
	if parent == nil || child == nil {
		return 0
	}
	for i, candidate := range parent.Children {
		if candidate == child {
			return i
		}
	}
	return 0
}

func cloneDocument(doc label.Document) label.Document {
	cloned := doc
	cloned.Elements = make([]label.Element, len(doc.Elements))
	for i, element := range doc.Elements {
		cloned.Elements[i] = cloneElement(element)
	}
	return cloned
}

func cloneElement(element label.Element) label.Element {
	cloned := element
	if element.Text != nil {
		text := *element.Text
		cloned.Text = &text
	}
	if element.QR != nil {
		qr := *element.QR
		cloned.QR = &qr
	}
	return cloned
}

func cloneElements(elements []label.Element) []label.Element {
	cloned := make([]label.Element, len(elements))
	for i, element := range elements {
		cloned[i] = cloneElement(element)
	}
	return cloned
}
