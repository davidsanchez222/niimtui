package tui

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
)

//go:embed printer_art/*.txt
var printerArtFiles embed.FS

type printerArtVariant struct {
	Name  string
	Lines []string
}

func printerArt(model string, variant int) []string {
	variants := printerArtVariants(model)
	if len(variants) == 0 {
		return nil
	}
	variant = positiveMod(variant, len(variants))
	return append([]string(nil), variants[variant].Lines...)
}

func printerArtVariantLabel(model string, variant int) string {
	variants := printerArtVariants(model)
	if len(variants) == 0 {
		return "none"
	}
	variant = positiveMod(variant, len(variants))
	return fmt.Sprintf("%s %d/%d", printerArtModelKey(model), variant+1, len(variants))
}

func printerArtVariantCount(model string) int {
	return len(printerArtVariants(model))
}

func printerArtVariants(model string) []printerArtVariant {
	key := printerArtModelKey(model)
	variants := loadPrinterArtVariants(key)
	if len(variants) > 0 {
		return variants
	}
	return loadPrinterArtVariants("generic")
}

func loadPrinterArtVariants(key string) []printerArtVariant {
	entries, err := printerArtFiles.ReadDir("printer_art")
	if err != nil {
		return nil
	}
	variants := make([]printerArtVariant, 0)
	prefix := key + "-"
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || path.Ext(entry.Name()) != ".txt" {
			continue
		}
		data, err := printerArtFiles.ReadFile(path.Join("printer_art", entry.Name()))
		if err != nil {
			continue
		}
		variants = append(variants, printerArtVariant{Name: strings.TrimSuffix(entry.Name(), ".txt"), Lines: splitPrinterArt(data)})
	}
	sort.Slice(variants, func(i, j int) bool {
		return variants[i].Name < variants[j].Name
	})
	return variants
}

func splitPrinterArt(data []byte) []string {
	text := strings.TrimRight(string(data), "\r\n")
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(text, "\n")
}

func printerArtModelKey(model string) string {
	model = strings.ToUpper(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(model, "B1"):
		return "b1"
	case strings.HasPrefix(model, "D110"), strings.HasPrefix(model, "D11"):
		return "d110"
	default:
		return "generic"
	}
}

func positiveMod(value, mod int) int {
	if mod <= 0 {
		return 0
	}
	value %= mod
	if value < 0 {
		value += mod
	}
	return value
}
