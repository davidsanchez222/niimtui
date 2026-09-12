package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/render"
)

func Run(widthMM, heightMM float64, shape, fontPath string, printConfig PrintConfig) error {
	if widthMM <= 0 || heightMM <= 0 {
		return fmt.Errorf("label width and height must be greater than zero")
	}
	if err := render.ValidateFontPath(fontPath); err != nil {
		return err
	}

	p := tea.NewProgram(
		NewModel(widthMM, heightMM, shape, fontPath, printConfig),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
