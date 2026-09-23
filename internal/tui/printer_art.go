package tui

import (
	"embed"
	"path"
	"sort"
	"strings"
)

//go:embed printer_art/*.txt
var printerArtFiles embed.FS

type printerArtAsset struct {
	Name  string
	Lines []string
}

func printerArt(model string) []string {
	art := printerArtForKey(printerArtModelKey(model))
	if len(art) == 0 {
		art = printerArtForKey("generic")
	}
	if len(art) == 0 {
		return nil
	}
	return append([]string(nil), art...)
}

func printerArtForKey(key string) []string {
	assets := loadPrinterArtAssets(key)
	if len(assets) == 0 {
		return nil
	}
	return assets[0].Lines
}

func loadPrinterArtAssets(key string) []printerArtAsset {
	entries, err := printerArtFiles.ReadDir("printer_art")
	if err != nil {
		return nil
	}
	assets := make([]printerArtAsset, 0)
	prefix := key
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || path.Ext(entry.Name()) != ".txt" {
			continue
		}
		data, err := printerArtFiles.ReadFile(path.Join("printer_art", entry.Name()))
		if err != nil {
			continue
		}
		assets = append(assets, printerArtAsset{Name: strings.TrimSuffix(entry.Name(), ".txt"), Lines: splitPrinterArt(data)})
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Name < assets[j].Name
	})
	return assets
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
