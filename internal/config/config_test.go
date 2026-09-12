package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	want := filepath.Join(dir, "niimtui", "config.json")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}
}

func TestSaveAndLoadDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	printer := PrinterProfile{
		Name:          "b1-default",
		Model:         "B1",
		Transport:     "ble",
		DeviceName:    "B1-Test",
		DefaultPreset: "b1-50x50-round",
	}
	presets := []LabelPreset{{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-title-subtitle", MarginsMM: 2}}
	cfg, err := DefaultConfig(printer, presets)
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}
	if loaded.Printers[0].Name != printer.Name {
		t.Fatalf("loaded printer = %q, want %q", loaded.Printers[0].Name, printer.Name)
	}
	if loaded.Server.AuthToken == "" {
		t.Fatal("loaded auth token is empty")
	}
}
