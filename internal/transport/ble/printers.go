package ble

import (
	"context"
	"fmt"
	"strings"
	"time"

	"niimtui/internal/config"
	"niimtui/internal/printtrace"
	"niimtui/internal/protocol/niimbot"
	"niimtui/internal/transport"
)

type printTask uint8

const (
	classicD110 printTask = iota
	d110MV4
	b1
)

// printerInfo contains the properties that affect raster preparation and
// print-task selection. A configured D110 with device type 2320 uses v4.
type printerInfo struct {
	task        printTask
	orientation niimbot.RasterOrientation
	density     int
}

func infoForPrinter(printer config.PrinterProfile, deviceType *int) (printerInfo, error) {
	switch strings.ToUpper(printer.Model) {
	case "D110":
		info := printerInfo{
			task:        classicD110,
			orientation: niimbot.RasterRotateLandscape,
			density:     max(0, min(printer.Defaults.Density, 5)),
		}
		if deviceType != nil && *deviceType == 2320 {
			info.task = d110MV4
		}
		return info, nil
	case "B1":
		density := printer.Defaults.Density
		if density <= 0 {
			density = 3
		}
		return printerInfo{task: b1, orientation: niimbot.RasterAsRendered, density: density}, nil
	default:
		return printerInfo{}, fmt.Errorf("niimbot packet session started but print task is not implemented yet for model %s", printer.Model)
	}
}

func (c *connection) Print(ctx context.Context, printer config.PrinterProfile, job transport.Job) error {
	if err := c.Prepare(ctx, printer); err != nil {
		return err
	}
	info, err := infoForPrinter(printer, c.deviceType)
	if err != nil {
		return err
	}
	raster, err := niimbot.PrepareRasterJob(job.Rendered, info.orientation)
	if err != nil {
		return err
	}
	switch info.task {
	case classicD110:
		return c.printD110(ctx, job, raster, info.density)
	case d110MV4:
		return c.printD110MV4(ctx, job, raster, info.density)
	case b1:
		return c.printB1(ctx, job, raster, info.density)
	default:
		return fmt.Errorf("unsupported print task for model %s", printer.Model)
	}
}

func (c *connection) printD110(ctx context.Context, job transport.Job, raster niimbot.RasterJob, density int) error {
	setupPackets := []struct {
		name string
		data []byte
		resp byte
	}{
		{name: "set_density", data: niimbot.SetDensityPacket(byte(density)), resp: niimbot.CmdSetDensity + 16},
		{name: "set_label_type", data: niimbot.SetLabelTypePacket(0x01), resp: niimbot.CmdSetLabelType + 16},
		{name: "print_start", data: niimbot.PrintStartPacket(), resp: niimbot.CmdPrintStart + 1},
		{name: "print_clear", data: niimbot.PrintClearPacket(), resp: niimbot.CmdPrintClear + 16},
		{name: "page_start", data: niimbot.PageStartPacket(), resp: niimbot.CmdPageStart + 1},
		{name: "page_size", data: niimbot.SetPageSizePacket(raster.HeightPx, raster.WidthPx), resp: niimbot.CmdSetPageSize + 1},
		{name: "print_quantity", data: niimbot.PrintQuantityPacket(job.Copies), resp: niimbot.CmdPrintQuantity + 1},
	}
	for _, packet := range setupPackets {
		if _, err := c.transceive(packet.name, packet.data, packet.resp, 10*time.Second); err != nil {
			return err
		}
		printtrace.Mark(ctx, packet.name+" acknowledged")
	}
	printtrace.Mark(ctx, "classic D110 setup complete; sending rows")
	if err := c.writeRows(ctx, raster.Rows); err != nil {
		return err
	}
	printtrace.Mark(ctx, "image rows sent")
	if err := c.finishPrint(ctx, 10*time.Second); err != nil {
		return err
	}
	c.setMeta("d110_job", map[string]any{
		"rows":            len(raster.Rows),
		"row_bytes":       raster.WidthPx / 8,
		"density":         byte(density),
		"label_type":      byte(0x01),
		"empty_row_runs":  true,
		"oriented_width":  raster.WidthPx,
		"oriented_height": raster.HeightPx,
	})
	return nil
}

func (c *connection) printD110MV4(ctx context.Context, job transport.Job, raster niimbot.RasterJob, density int) error {
	setupPackets := []struct {
		name string
		data []byte
		resp byte
	}{
		{name: "set_density", data: niimbot.SetDensityPacket(byte(density)), resp: niimbot.CmdSetDensity + 16},
		{name: "set_label_type", data: niimbot.SetLabelTypePacket(0x01), resp: niimbot.CmdSetLabelType + 16},
		{name: "print_start_v4", data: niimbot.PrintStartV4Packet(1, 0, 1), resp: niimbot.CmdPrintStart + 1},
	}
	for _, packet := range setupPackets {
		if _, err := c.transceive(packet.name, packet.data, packet.resp, 2*time.Second); err != nil {
			return err
		}
		printtrace.Mark(ctx, packet.name+" acknowledged")
	}
	if err := c.writePacket("print_status_one_way", niimbot.PrintStatusPacket()); err != nil {
		return err
	}
	if _, err := c.transceive("page_size_v4", niimbot.SetPageSizeV4Packet(raster.HeightPx, raster.WidthPx, job.Copies), niimbot.CmdSetPageSize+1, 2*time.Second); err != nil {
		return err
	}
	printtrace.Mark(ctx, "page_size_v4 acknowledged")
	printtrace.Mark(ctx, "D110_M v4 setup complete; sending rows")
	if err := c.writeRows(ctx, raster.Rows); err != nil {
		return err
	}
	printtrace.Mark(ctx, "image rows sent")
	if err := c.finishPrint(ctx, 2*time.Second); err != nil {
		return err
	}
	_ = c.writePacket("heartbeat_one_way", niimbot.HeartbeatPacket())
	c.setMeta("d110_job", map[string]any{
		"rows":            len(raster.Rows),
		"row_bytes":       raster.WidthPx / 8,
		"density":         byte(density),
		"label_type":      byte(0x01),
		"empty_row_runs":  true,
		"oriented_width":  raster.WidthPx,
		"oriented_height": raster.HeightPx,
		"variant":         "d110_m_v4",
	})
	return nil
}

func (c *connection) printB1(ctx context.Context, job transport.Job, raster niimbot.RasterJob, density int) error {
	setupPackets := []struct {
		name string
		data []byte
		resp byte
	}{
		{name: "set_density", data: niimbot.SetDensityPacket(byte(density)), resp: niimbot.CmdSetDensity + 16},
		{name: "set_label_type", data: niimbot.SetLabelTypePacket(0x01), resp: niimbot.CmdSetLabelType + 16},
		{name: "print_start_pages", data: niimbot.PrintStartPagesPacket(1, 0), resp: niimbot.CmdPrintStart + 1},
		{name: "page_start", data: niimbot.PageStartPacket(), resp: niimbot.CmdPageStart + 1},
		{name: "page_size_pages", data: niimbot.SetPageSizePagesPacket(raster.HeightPx, raster.WidthPx, job.Copies), resp: niimbot.CmdSetPageSize + 1},
	}
	for _, packet := range setupPackets {
		if _, err := c.transceive(packet.name, packet.data, packet.resp, 2*time.Second); err != nil {
			return err
		}
		printtrace.Mark(ctx, packet.name+" acknowledged")
	}
	printtrace.Mark(ctx, "B1 setup complete; sending rows")
	if err := c.writeRows(ctx, raster.Rows); err != nil {
		return err
	}
	printtrace.Mark(ctx, "image rows sent")
	if err := c.finishPrint(ctx, 2*time.Second); err != nil {
		return err
	}
	c.setMeta("b1_job", map[string]any{
		"rows":      len(raster.Rows),
		"row_bytes": len(raster.Rows[0]),
		"density":   density,
		"width_px":  raster.WidthPx,
		"height_px": raster.HeightPx,
	})
	return nil
}

func (c *connection) finishPrint(ctx context.Context, pageEndTimeout time.Duration) error {
	if _, err := c.transceive("page_end", niimbot.PageEndPacket(), niimbot.CmdPageEnd+1, pageEndTimeout); err != nil {
		return err
	}
	printtrace.Mark(ctx, "page end acknowledged")
	if err := c.waitForPrintStatus(); err != nil {
		return err
	}
	printtrace.Mark(ctx, "print completion status reached")
	for i := 0; i < 6; i++ {
		payload, err := c.transceive("print_end", niimbot.PrintEndPacket(), niimbot.CmdPrintEnd+1, 2*time.Second)
		if err != nil {
			return err
		}
		if len(payload) > 0 && payload[0] != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
