package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"niimtui/internal/transport"
)

func TestScanPickerSortsNamedDevicesBeforeUnknownDevices(t *testing.T) {
	picker := newScanPicker(nil, nil)
	picker.devices["unknown-b"] = transport.ScanResult{Address: "unknown-b", RSSI: -70}
	picker.devices["z"] = transport.ScanResult{Address: "z", Name: "Zebra", RSSI: -60}
	picker.devices["a"] = transport.ScanResult{Address: "a", Name: "alpha", RSSI: -50}
	picker.devices["unknown-a"] = transport.ScanResult{Address: "unknown-a", RSSI: -80}
	picker.showUnknown = true
	picker.sortDevices()

	got := []string{
		picker.options()[0].device.Address,
		picker.options()[1].device.Address,
		picker.options()[2].device.Address,
		picker.options()[3].device.Address,
	}
	want := []string{"a", "z", "unknown-a", "unknown-b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered addresses = %v, want %v", got, want)
		}
	}
}

func TestScanPickerManualOptionIsLast(t *testing.T) {
	picker := newScanPicker(nil, nil)
	picker.devices["b"] = transport.ScanResult{Address: "b", Name: "B1", RSSI: -50}
	picker.devices["unknown-a"] = transport.ScanResult{Address: "unknown-a", RSSI: -80}
	picker.sortDevices()

	lines := picker.optionLines()
	if got := lines[len(lines)-1]; got != "Enter manually" {
		t.Fatalf("last option = %q, want Enter manually", got)
	}
	if strings.Contains(lines[0], "Unknown device") {
		t.Fatalf("first option = %q, want named device", lines[0])
	}
}

func TestScanPickerCollapsesUnknownDevicesByDefault(t *testing.T) {
	picker := newScanPicker(nil, nil)
	picker.devices["unknown-a"] = transport.ScanResult{Address: "unknown-a", RSSI: -80}
	picker.devices["unknown-b"] = transport.ScanResult{Address: "unknown-b", RSSI: -70}
	picker.sortDevices()

	lines := picker.optionLines()
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want hidden summary and manual option", len(lines))
	}
	if got := lines[0]; got != "Unknown devices hidden (2) - press u to show" {
		t.Fatalf("summary line = %q", got)
	}
	if strings.Contains(strings.Join(lines, "\n"), "unknown-a") {
		t.Fatalf("unknown device address rendered while collapsed: %v", lines)
	}
}

func TestScanPickerShowsUnknownDevicesWhenToggled(t *testing.T) {
	picker := newScanPicker(nil, nil)
	picker.showUnknown = true
	picker.devices["unknown-b"] = transport.ScanResult{Address: "unknown-b", RSSI: -70}
	picker.devices["unknown-a"] = transport.ScanResult{Address: "unknown-a", RSSI: -80}
	picker.sortDevices()

	lines := picker.optionLines()
	if !strings.Contains(lines[0], "unknown-a") || !strings.Contains(lines[1], "unknown-b") {
		t.Fatalf("unknown devices not sorted/rendered by address: %v", lines)
	}
}

func TestScanPickerPrioritizesKnownNiimbotModels(t *testing.T) {
	picker := newScanPicker(nil, nil)
	picker.devices["zebra"] = transport.ScanResult{Address: "zebra", Name: "AAA Speaker", RSSI: -50}
	picker.devices["d110"] = transport.ScanResult{Address: "d110", Name: "D110_M-H913040249", RSSI: -60}
	picker.devices["b1"] = transport.ScanResult{Address: "b1", Name: "B1-I427031488", RSSI: -55}
	picker.sortDevices()

	options := picker.options()
	got := []string{options[0].device.Address, options[1].device.Address, options[2].device.Address}
	want := []string{"b1", "d110", "zebra"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered addresses = %v, want %v", got, want)
		}
	}
}

func TestKnownNiimbotRankMatchesOnlyPrefixWithSeparator(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "B1-I427031488", want: true},
		{name: "D110_M-H913040249", want: true},
		{name: "B21 Pro-123", want: true},
		{name: "B3S ABC", want: true},
		{name: "Speaker B1", want: false},
		{name: "B100", want: false},
	}
	for _, tc := range cases {
		_, got := knownNiimbotRank(tc.name)
		if got != tc.want {
			t.Fatalf("knownNiimbotRank(%q) known = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestScanPickerAbortKeysSwitchToManualMode(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}
	for _, key := range keys {
		model, _ := newScanPicker(nil, nil).Update(key)
		picker, ok := model.(scanPicker)
		if !ok {
			t.Fatalf("updated model type = %T, want scanPicker", model)
		}
		if !picker.aborted {
			t.Fatalf("key %q did not set aborted", key.String())
		}
	}
}

func TestScanPickerKeyNavigationStaysWithinOptions(t *testing.T) {
	picker := newScanPicker(nil, nil)
	picker.devices["a"] = transport.ScanResult{Address: "a", Name: "Alpha", RSSI: -50}
	picker.devices["b"] = transport.ScanResult{Address: "b", Name: "Bravo", RSSI: -60}
	picker.sortDevices()

	model, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	picker = model.(scanPicker)
	if picker.cursor != 1 {
		t.Fatalf("cursor after j = %d, want 1", picker.cursor)
	}

	model, _ = picker.Update(tea.KeyMsg{Type: tea.KeyDown})
	picker = model.(scanPicker)
	if picker.cursor != 2 {
		t.Fatalf("cursor after down = %d, want 2", picker.cursor)
	}

	model, _ = picker.Update(tea.KeyMsg{Type: tea.KeyDown})
	picker = model.(scanPicker)
	if picker.cursor != 2 {
		t.Fatalf("cursor after down at end = %d, want 2", picker.cursor)
	}

	model, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	picker = model.(scanPicker)
	if picker.cursor != 1 {
		t.Fatalf("cursor after k = %d, want 1", picker.cursor)
	}
}
