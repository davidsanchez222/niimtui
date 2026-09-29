package main

import (
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"niimtui/internal/api"
	"niimtui/internal/config"
	"niimtui/internal/label"
)

func testLabelConfig(t *testing.T) string {
	t.Helper()
	doc := label.NewDocument(50, 50)
	doc.Shape = "round"
	doc.Elements = []label.Element{{ID: "qr-1", Type: label.ElementQR, XMM: 10, YMM: 10, WidthMM: 30, HeightMM: 30, QR: &label.QRElement{Value: "saved"}}}
	cfg := config.Config{
		Server:        config.ServerConfig{Listen: "127.0.0.1:8443", AuthToken: "test"},
		ActivePrinter: "b1",
		Printers:      []config.PrinterProfile{{Name: "b1", Model: "B1", Transport: "ble", DeviceName: "B1", DefaultPreset: "round"}},
		Presets:       []config.LabelPreset{{Name: "round", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-only"}, {Name: "small", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-only"}},
		DesignPresets: []config.DesignPreset{{Name: "box", Document: doc, Bindings: []config.DesignBinding{{Name: "url", ElementID: "qr-1", Required: true}}}},
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path
}

func captureCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	runErr := run(args)
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), runErr
}

func TestDesignsAndPreviewWithBindings(t *testing.T) {
	path := testLabelConfig(t)
	output, err := captureCLI(t, "designs", "--config", path)
	if err != nil {
		t.Fatal(err)
	}
	var designs []struct {
		Name     string `json:"name"`
		Bindings []struct {
			Name     string `json:"name"`
			Required bool   `json:"required"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal([]byte(output), &designs); err != nil {
		t.Fatal(err)
	}
	if len(designs) != 1 || designs[0].Name != "box" || len(designs[0].Bindings) != 1 || !designs[0].Bindings[0].Required {
		t.Fatalf("designs = %s", output)
	}
	out := filepath.Join(t.TempDir(), "preview.png")
	output, err = captureCLI(t, "preview", "--config", path, "--design", "box", "--set", "url=https://example.com/a=b", "--out", out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"ok": true`) {
		t.Fatalf("preview result = %s", output)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
	other := filepath.Join(t.TempDir(), "different-roll.png")
	output, err = captureCLI(t, "preview", "--config", path, "--design", "box", "--set", "url=other", "--preset", "small", "--out", other)
	if err != nil || !strings.Contains(output, `"ok": true`) {
		t.Fatalf("preview with different roll: %v, %s", err, output)
	}
	qrOut := filepath.Join(t.TempDir(), "qr.png")
	output, err = captureCLI(t, "preview", "--config", path, "--qr", "https://example.com", "--title", "Box", "--out", qrOut)
	if err != nil || !strings.Contains(output, `"ok": true`) {
		t.Fatalf("quick QR preview: %v, %s", err, output)
	}
}

func TestPrintValidatesBindingsAndStockBeforeConnecting(t *testing.T) {
	path := testLabelConfig(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--design", "box"}, "requires --set url"},
		{[]string{"--design", "box", "--set", "url=test", "--preset", "small"}, "does not match"},
		{[]string{"--design", "box", "--set", "url=test", "--set", "typo=x"}, "no binding"},
	} {
		args := append([]string{"print", "--config", path}, tc.args...)
		output, err := captureCLI(t, args...)
		var exit *commandExit
		if !errors.As(err, &exit) || exit.code == 0 {
			t.Fatalf("run(%v) error = %v", args, err)
		}
		if !strings.Contains(output, tc.want) {
			t.Fatalf("run(%v) output = %s, want %q", args, output, tc.want)
		}
	}
}

func TestFailedPrintResponseExitsNonzero(t *testing.T) {
	output, err := capturePrintFailure(t)
	var exit *commandExit
	if !errors.As(err, &exit) || exit.code != 1 {
		t.Fatalf("exit = %v", err)
	}
	if !strings.Contains(output, `"ok": false`) {
		t.Fatalf("response = %s", output)
	}
}

func capturePrintFailure(t *testing.T) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	result := reportPrint(api.PrintResponse{OK: false, Error: &api.ErrorBody{Code: "PRINT_FAILED", Message: "offline"}})
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), result
}
