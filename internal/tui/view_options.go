package tui

func (m *Model) toggleGrid() bool {
	m.Grid = !m.Grid
	if m.Grid {
		m.setStatus("Grid on.")
		return true
	}
	m.setStatus("Grid off.")
	return true
}

func (m *Model) toggleInvertedColors() bool {
	m.Document.Inverted = !m.Document.Inverted
	m.commitHistory("invert colors")
	if m.Document.Inverted {
		m.setStatus("Inverted black/white colors on.")
		return true
	}
	m.setStatus("Inverted black/white colors off.")
	return true
}
