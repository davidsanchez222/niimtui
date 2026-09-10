package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"niimcli/internal/config"
)

//go:embed data/niimbot-label-sizes.json
var catalogFS embed.FS

var knownModels = []string{
	"M2",
	"M3",
	"N1",
	"B21 Pro",
	"B1",
	"B4",
	"B2 Pro",
	"B1 Pro",
	"B2",
	"B21",
	"B3S",
	"K3",
	"D11",
	"D110",
	"D101",
}

type LabelSize struct {
	WidthMM  float64 `json:"width_mm"`
	HeightMM float64 `json:"height_mm"`
	Shape    string  `json:"shape"`
	Lanes    int     `json:"lanes,omitempty"`
}

type rawCatalog struct {
	Notes  []string     `json:"notes"`
	Groups []labelGroup `json:"groups"`
}

type labelGroup struct {
	CompatibleModels []string    `json:"compatible_models"`
	Sizes            []LabelSize `json:"sizes"`
}

var (
	loadOnce sync.Once
	loaded   rawCatalog
	loadErr  error
)

func KnownModels() []string {
	models := make([]string, len(knownModels))
	copy(models, knownModels)
	return models
}

func LabelSizesForModel(model string) ([]LabelSize, error) {
	c, err := load()
	if err != nil {
		return nil, err
	}
	var sizes []LabelSize
	seen := make(map[string]struct{})
	for _, group := range c.Groups {
		if !containsModel(group.CompatibleModels, model) {
			continue
		}
		for _, size := range group.Sizes {
			key := sizeKey(size)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sizes = append(sizes, size)
		}
	}
	sort.SliceStable(sizes, func(i, j int) bool {
		if sizes[i].WidthMM != sizes[j].WidthMM {
			return sizes[i].WidthMM < sizes[j].WidthMM
		}
		if sizes[i].HeightMM != sizes[j].HeightMM {
			return sizes[i].HeightMM < sizes[j].HeightMM
		}
		if sizes[i].Shape != sizes[j].Shape {
			return sizes[i].Shape < sizes[j].Shape
		}
		return sizes[i].Lanes < sizes[j].Lanes
	})
	return sizes, nil
}

func LabelPresetsForModel(model string) ([]config.LabelPreset, error) {
	sizes, err := LabelSizesForModel(model)
	if err != nil {
		return nil, err
	}
	presets := make([]config.LabelPreset, 0, len(sizes))
	for _, size := range sizes {
		presets = append(presets, config.LabelPreset{
			Name:      PresetName(model, size),
			WidthMM:   size.WidthMM,
			HeightMM:  size.HeightMM,
			Shape:     normalizedShape(size.Shape),
			Layout:    "qr-title-subtitle",
			MarginsMM: defaultMargins(size),
		})
	}
	return presets, nil
}

func PresetName(model string, size LabelSize) string {
	parts := []string{slug(model), dimension(size.WidthMM) + "x" + dimension(size.HeightMM)}
	shape := normalizedShape(size.Shape)
	if shape != "" && shape != "rect" {
		parts = append(parts, shape)
	}
	if size.Lanes > 1 {
		parts = append(parts, fmt.Sprintf("%dlane", size.Lanes))
	}
	return strings.Join(parts, "-")
}

func load() (rawCatalog, error) {
	loadOnce.Do(func() {
		data, err := catalogFS.ReadFile("data/niimbot-label-sizes.json")
		if err != nil {
			loadErr = fmt.Errorf("read label catalog: %w", err)
			return
		}
		if err := json.Unmarshal(data, &loaded); err != nil {
			loadErr = fmt.Errorf("parse label catalog: %w", err)
		}
	})
	return loaded, loadErr
}

func containsModel(models []string, model string) bool {
	model = strings.TrimSpace(model)
	for _, candidate := range models {
		if strings.EqualFold(strings.TrimSpace(candidate), model) {
			return true
		}
	}
	return false
}

func sizeKey(size LabelSize) string {
	return fmt.Sprintf("%s:%s:%s:%d", dimension(size.WidthMM), dimension(size.HeightMM), normalizedShape(size.Shape), size.Lanes)
}

func normalizedShape(shape string) string {
	shape = strings.ToLower(strings.TrimSpace(shape))
	if shape == "" {
		return "rect"
	}
	return shape
}

func defaultMargins(size LabelSize) float64 {
	if math.Min(size.WidthMM, size.HeightMM) <= 15 {
		return 1
	}
	return 2
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func dimension(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
