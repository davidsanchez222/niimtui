package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"niimtui/internal/config"
)

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func savedPrintCommand(item galleryItem, printer config.PrinterProfile, stock config.LabelPreset, configPath string) (string, error) {
	if !item.Saved {
		return "", fmt.Errorf("save this starter design before generating a print command")
	}
	if printer.Name == "" || !config.MatchesStock(item.Document, stock) {
		return "", fmt.Errorf("select a printer and a matching installed label roll for %q", item.Name)
	}
	args := []string{"niimtui", "print"}
	if configPath != "" {
		args = append(args, "--config", shellQuote(configPath))
	}
	args = append(args, "--printer", shellQuote(printer.Name), "--preset", shellQuote(stock.Name), "--design", shellQuote(item.Name))
	bindings := append([]config.DesignBinding(nil), item.Bindings...)
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Name < bindings[j].Name })
	for _, binding := range bindings {
		if !binding.Required {
			continue
		}
		value := ""
		for _, element := range item.Document.Elements {
			if element.ID != binding.ElementID {
				continue
			}
			if element.Text != nil {
				value = element.Text.Value
			} else if element.QR != nil {
				value = element.QR.Value
			}
			break
		}
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("required binding %q has no saved value; edit and save the design first", binding.Name)
		}
		args = append(args, "--set", shellQuote(binding.Name+"="+value))
	}
	return strings.Join(args, " "), nil
}

func (m *Model) showGalleryCommand() {
	item, ok := m.currentGalleryItem()
	if !ok {
		return
	}
	var printer config.PrinterProfile
	for _, candidate := range m.Print.Printers {
		if candidate.Name == m.Print.Printer {
			printer = candidate
			break
		}
	}
	var stock config.LabelPreset
	if m.Preset >= 0 && m.Preset < len(m.Presets) && config.MatchesStock(item.Document, m.Presets[m.Preset]) {
		stock = m.Presets[m.Preset]
	} else {
		for _, candidate := range m.Presets {
			if config.MatchesStock(item.Document, candidate) {
				stock = candidate
				break
			}
		}
	}
	path := m.Print.ConfigPath
	if path != "" {
		if defaultPath, err := config.DefaultPath(); err == nil {
			resolved, pathErr := filepath.Abs(path)
			defaultResolved, defaultErr := filepath.Abs(defaultPath)
			if pathErr == nil && defaultErr == nil && resolved == defaultResolved {
				path = ""
			}
		}
	}
	command, err := savedPrintCommand(item, printer, stock, path)
	if err != nil {
		m.setStatus("Print command: %v", err)
		return
	}
	m.Prompt = PromptState{Mode: PromptCommand, Value: command}
	m.CommandScroll = 0
	m.setStatus("Print command for %q. Press c to copy, esc to close.", item.Name)
}
