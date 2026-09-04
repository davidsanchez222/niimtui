package tui

import (
	"os"

	"niimcli/internal/render"
)

const previewOutputPath = "preview.png"

func (m *Model) exportPreview() bool {
	result, err := render.RenderDocument(m.Document)
	if err != nil {
		m.setStatus("Preview render failed: %v", err)
		return true
	}
	if err := os.WriteFile(previewOutputPath, result.PreviewPNG, 0o644); err != nil {
		m.setStatus("Preview write failed: %v", err)
		return true
	}
	m.setStatus("Preview written to %s (%dx%d).", previewOutputPath, result.WidthPx, result.HeightPx)
	return true
}
