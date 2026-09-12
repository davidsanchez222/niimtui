package render

import (
	"fmt"
	"math"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"niimtui/internal/label"
)

const textPaddingMM = 1.0

const textThreshold = 192

type TextLayout struct {
	Lines          []string
	LineHeightPx   int
	AscentPx       int
	BlockHeightPx  int
	MaxLineWidthPx int
}

func LayoutText(text string, fontSize float64, maxWidthPx int) (TextLayout, error) {
	return LayoutTextWithFontPath(text, fontSize, "", maxWidthPx)
}

func LayoutTextWithFontPath(text string, fontSize float64, fontPath string, maxWidthPx int) (TextLayout, error) {
	face, err := newTextFace(fontSize, fontPath)
	if err != nil {
		return TextLayout{}, err
	}
	defer face.Close()
	return layoutTextWithFace(text, face, maxWidthPx), nil
}

func MinimumTextWidthMM(element label.Element) (float64, error) {
	if element.Text == nil {
		return 0, nil
	}
	face, err := newTextFace(element.Text.FontSize, element.Text.FontPath)
	if err != nil {
		return 0, err
	}
	defer face.Close()
	widthPx := max(font.MeasureString(face, "W").Ceil(), face.Metrics().Height.Ceil()/2) + textPaddingPx()*2
	return pxToMM(widthPx), nil
}

func RequiredTextHeightMM(element label.Element, widthMM float64) (float64, error) {
	if element.Text == nil || strings.TrimSpace(element.Text.Value) == "" {
		return 0, nil
	}
	innerWidthPx := max(1, mmToPx(widthMM)-textPaddingPx()*2)
	layout, err := LayoutTextWithFontPath(element.Text.Value, element.Text.FontSize, element.Text.FontPath, innerWidthPx)
	if err != nil {
		return 0, err
	}
	return pxToMM(layout.BlockHeightPx + textPaddingPx()*2), nil
}

func textPaddingPx() int {
	return max(1, int(math.Round(textPaddingMM*dotsPerMM)))
}

func ValidateFontPath(fontPath string) error {
	if strings.TrimSpace(fontPath) == "" {
		return nil
	}
	if _, err := loadFont(fontPath); err != nil {
		return err
	}
	return nil
}

func newTextFace(fontSize float64, fontPath string) (font.Face, error) {
	loadedFont, err := loadFont(fontPath)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(loadedFont, &opentype.FaceOptions{
		Size:    maxFloat(fontSize, 8),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create font face: %w", err)
	}
	return face, nil
}

func loadFont(fontPath string) (*opentype.Font, error) {
	fontPath = strings.TrimSpace(fontPath)
	if fontPath == "" {
		return textFont, nil
	}
	data, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("read font %q: %w", fontPath, err)
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font %q: %w", fontPath, err)
	}
	return parsed, nil
}

func layoutTextWithFace(text string, face font.Face, maxWidthPx int) TextLayout {
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	if lineHeight <= 0 {
		lineHeight = 1
	}
	if maxWidthPx <= 0 {
		maxWidthPx = 1
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return TextLayout{LineHeightPx: lineHeight, AscentPx: metrics.Ascent.Ceil()}
	}

	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return TextLayout{LineHeightPx: lineHeight, AscentPx: metrics.Ascent.Ceil()}
	}

	lines := make([]string, 0, len(words))
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			if font.MeasureString(face, current).Ceil() <= maxWidthPx {
				continue
			}
			parts := breakLongWord(face, current, maxWidthPx)
			lines = append(lines, parts[:len(parts)-1]...)
			current = parts[len(parts)-1]
			continue
		}

		candidate := current + " " + word
		if font.MeasureString(face, candidate).Ceil() <= maxWidthPx {
			current = candidate
			continue
		}

		lines = append(lines, current)
		current = word
		if font.MeasureString(face, current).Ceil() <= maxWidthPx {
			continue
		}
		parts := breakLongWord(face, current, maxWidthPx)
		lines = append(lines, parts[:len(parts)-1]...)
		current = parts[len(parts)-1]
	}
	if current != "" {
		lines = append(lines, current)
	}

	maxLineWidth := 0
	for _, line := range lines {
		maxLineWidth = max(maxLineWidth, font.MeasureString(face, line).Ceil())
	}

	return TextLayout{
		Lines:          lines,
		LineHeightPx:   lineHeight,
		AscentPx:       metrics.Ascent.Ceil(),
		BlockHeightPx:  len(lines) * lineHeight,
		MaxLineWidthPx: maxLineWidth,
	}
}

func breakLongWord(face font.Face, word string, maxWidthPx int) []string {
	runes := []rune(word)
	parts := make([]string, 0, len(runes))
	start := 0
	for start < len(runes) {
		end := start + 1
		for end <= len(runes) {
			segment := string(runes[start:end])
			if font.MeasureString(face, segment).Ceil() > maxWidthPx {
				if end == start+1 {
					parts = append(parts, segment)
					start = end
				} else {
					parts = append(parts, string(runes[start:end-1]))
					start = end - 1
				}
				break
			}
			end++
		}
		if end > len(runes) {
			parts = append(parts, string(runes[start:]))
			break
		}
	}
	return parts
}

func pxToMM(px int) float64 {
	return float64(px) / dotsPerMM
}
