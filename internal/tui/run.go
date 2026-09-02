package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(widthMM, heightMM float64) error {
	if widthMM <= 0 || heightMM <= 0 {
		return fmt.Errorf("label width and height must be greater than zero")
	}

	p := tea.NewProgram(
		NewModel(widthMM, heightMM),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
