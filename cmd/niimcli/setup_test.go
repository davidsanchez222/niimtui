package main

import (
	"strings"
	"testing"

	"niimcli/internal/transport"
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
