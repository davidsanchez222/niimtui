package config

import (
	"fmt"
	"strings"

	"niimtui/internal/label"
)

// Bind returns a copy of the saved document with named values substituted.
// Required bindings must be supplied even if the saved element has a value.
func (preset DesignPreset) Bind(values map[string]string) (label.Document, error) {
	doc := preset.Document
	doc.Elements = append([]label.Element(nil), doc.Elements...)
	byName := make(map[string]DesignBinding, len(preset.Bindings))
	for _, binding := range preset.Bindings {
		byName[binding.Name] = binding
		if binding.Required && strings.TrimSpace(values[binding.Name]) == "" {
			return label.Document{}, fmt.Errorf("design %q requires --set %s=...", preset.Name, binding.Name)
		}
	}
	for name, value := range values {
		binding, ok := byName[name]
		if !ok {
			return label.Document{}, fmt.Errorf("design %q has no binding %q", preset.Name, name)
		}
		found := false
		for i := range doc.Elements {
			element := &doc.Elements[i]
			if element.ID != binding.ElementID {
				continue
			}
			found = true
			switch {
			case element.Text != nil:
				text := *element.Text
				text.Value = value
				element.Text = &text
			case element.QR != nil:
				qr := *element.QR
				qr.Value = value
				element.QR = &qr
			default:
				return label.Document{}, fmt.Errorf("binding %q has no text or QR target", name)
			}
			break
		}
		if !found {
			return label.Document{}, fmt.Errorf("binding %q refers to missing element %q", name, binding.ElementID)
		}
	}
	return doc, nil
}

func MatchesStock(doc label.Document, stock LabelPreset) bool {
	return doc.WidthMM == stock.WidthMM && doc.HeightMM == stock.HeightMM && strings.EqualFold(doc.Shape, stock.Shape)
}
