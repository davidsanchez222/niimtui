package config

import (
	"strings"
	"testing"

	"niimtui/internal/label"
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
