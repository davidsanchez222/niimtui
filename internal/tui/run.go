package tui

import (
	"fmt"
	"os"

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

	options := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	if os.Getenv("NIIMTUI_DEBUG_PANIC") == "1" {
		options = append(options, tea.WithoutCatchPanics())
	}

	p := tea.NewProgram(NewModel(widthMM, heightMM, shape, fontPath, printConfig), options...)

	_, err := p.Run()
	return err
}
