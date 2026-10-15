package config

import (
	"strings"
	"testing"

	"github.com/davidsanchez222/niimtui/internal/label"
)

func TestDesignBindingsCopyDefaultsAndRequired(t *testing.T) {
	doc := label.NewDocument(50, 30)
	doc.Elements = []label.Element{
		{ID: "qr-1", Type: label.ElementQR, QR: &label.QRElement{Value: "saved-url"}},
		{ID: "text-2", Type: label.ElementText, Text: &label.TextElement{Value: "Saved title"}},
	}
	preset := DesignPreset{Name: "box", Document: doc, Bindings: []DesignBinding{
		{Name: "url", ElementID: "qr-1", Required: true},
		{Name: "title", ElementID: "text-2"},
	}}
	if _, err := preset.Bind(nil); err == nil || !strings.Contains(err.Error(), "requires --set url") {
		t.Fatalf("missing required binding error = %v", err)
	}
	bound, err := preset.Bind(map[string]string{"url": "new-url"})
	if err != nil {
		t.Fatal(err)
	}
	if bound.Elements[0].QR.Value != "new-url" || bound.Elements[1].Text.Value != "Saved title" {
		t.Fatalf("bound document = %#v", bound.Elements)
	}
	if doc.Elements[0].QR.Value != "saved-url" {
		t.Fatal("binding mutated saved document")
	}
	if _, err := preset.Bind(map[string]string{"url": "new-url", "typo": "x"}); err == nil {
		t.Fatal("unknown binding was accepted")
	}
}

func TestValidateRejectsBrokenDesignBindings(t *testing.T) {
	base := Config{
		Server:   ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test"},
		Printers: []PrinterProfile{{Name: "b1", Model: "B1", Transport: "ble", DeviceName: "B1", DefaultPreset: "round"}},
		Presets:  []LabelPreset{{Name: "round", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-only"}},
	}
	doc := label.NewDocument(50, 50)
	doc.Elements = []label.Element{{ID: "qr-1", Type: label.ElementQR, QR: &label.QRElement{Value: "saved"}}}
	base.DesignPresets = []DesignPreset{{Name: "box", Document: doc, Bindings: []DesignBinding{{Name: "url", ElementID: "missing"}}}}
	if err := base.Validate(); err == nil {
		t.Fatal("missing binding target was accepted")
	}
	base.DesignPresets[0].Bindings = []DesignBinding{{Name: "url", ElementID: "qr-1"}, {Name: "url", ElementID: "qr-1"}}
	if err := base.Validate(); err == nil {
		t.Fatal("duplicate binding was accepted")
	}
}

func TestMatchesStockUsesPhysicalDimensionsAfterCanvasRotation(t *testing.T) {
	stock := LabelPreset{WidthMM: 50, HeightMM: 80, Shape: "rect"}
	tests := []struct {
		name     string
		width    float64
		height   float64
		rotation int
		want     bool
	}{
		{name: "original", width: 50, height: 80, want: true},
		{name: "clockwise", width: 80, height: 50, rotation: 90, want: true},
		{name: "upside down", width: 50, height: 80, rotation: 180, want: true},
		{name: "counterclockwise", width: 80, height: 50, rotation: 270, want: true},
		{name: "different stock", width: 80, height: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := label.NewDocument(tt.width, tt.height)
			doc.Rotation = tt.rotation
			if got := MatchesStock(doc, stock); got != tt.want {
				t.Fatalf("MatchesStock() = %t, want %t", got, tt.want)
			}
		})
	}
}
