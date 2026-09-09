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
	picker.sortDevices()

	got := []string{
		picker.ordered[0].Address,
		picker.ordered[1].Address,
		picker.ordered[2].Address,
		picker.ordered[3].Address,
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
	picker.sortDevices()

	lines := picker.optionLines()
	if got := lines[len(lines)-1]; got != "Enter manually" {
		t.Fatalf("last option = %q, want Enter manually", got)
	}
	if strings.Contains(lines[0], "Unknown device") {
		t.Fatalf("first option = %q, want named device", lines[0])
	}
}
