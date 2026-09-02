package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"niimcli/internal/label"
)

type Model struct {
	Width  int
	Height int

	Document label.Document
	Canvas   Canvas

	Status string
	Ready  bool
}

func NewModel(widthMM, heightMM float64) Model {
	doc := label.NewDocument(widthMM, heightMM)
	return Model{
		Document: doc,
		Status:   fmt.Sprintf("Label %.1fmm x %.1fmm", widthMM, heightMM),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
