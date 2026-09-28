package ble

import (
	"testing"

	"niimtui/internal/config"
	"niimtui/internal/protocol/niimbot"
)

func TestInfoForPrinterSelectsVerifiedPrintTasks(t *testing.T) {
	v4Type := 2320
	b1Type := 4096
	tests := []struct {
		name        string
		model       string
		deviceType  *int
		density     int
		wantTask    printTask
		wantRaster  niimbot.RasterOrientation
		wantDensity int
		wantError   bool
	}{
		{name: "classic D110", model: "D110", density: 9, wantTask: classicD110, wantRaster: niimbot.RasterRotateLandscape, wantDensity: 5},
		{name: "D110_M v4", model: "D110", deviceType: &v4Type, density: 3, wantTask: d110MV4, wantRaster: niimbot.RasterRotateLandscape, wantDensity: 3},
		{name: "B1", model: "B1", deviceType: &b1Type, wantTask: b1, wantRaster: niimbot.RasterAsRendered, wantDensity: 3},
		{name: "unsupported B21", model: "B21", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := infoForPrinter(config.PrinterProfile{
				Model:    tt.model,
				Defaults: config.PrinterDefaults{Density: tt.density},
			}, tt.deviceType)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected unsupported model error")
				}
				return
			}
			if err != nil || info.task != tt.wantTask || info.orientation != tt.wantRaster || info.density != tt.wantDensity {
				t.Fatalf("infoForPrinter() = %+v, %v; want task %v, orientation %v, density %d", info, err, tt.wantTask, tt.wantRaster, tt.wantDensity)
			}
		})
	}
}
