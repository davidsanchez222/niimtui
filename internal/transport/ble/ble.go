package ble

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"niimtui/internal/config"
	"niimtui/internal/protocol/niimbot"
	"niimtui/internal/transport"

	"tinygo.org/x/bluetooth"
)

const (
	defaultScanTimeout = 5 * time.Second
	writeInterval      = 10 * time.Millisecond
)

type Backend struct {
	adapter *bluetooth.Adapter

	enableOnce sync.Once
	enableErr  error
}

func New() *Backend {
	return &Backend{adapter: bluetooth.DefaultAdapter}
}

func (b *Backend) Connect(ctx context.Context, printer config.PrinterProfile) (transport.Connection, error) {
	if err := b.enable(); err != nil {
		return nil, fmt.Errorf("enable ble adapter: %w", err)
	}

	var (
		address     bluetooth.Address
		name        string
		connectMode string
		err         error
	)

	if printer.Identifier != "" {
		address.Set(printer.Identifier)
		device, directErr := b.adapter.Connect(address, bluetooth.ConnectionParams{})
		if directErr == nil {
			return &connection{
				device: device,
				meta: map[string]any{
					"address":      address.String(),
					"matched_name": printer.DeviceName,
					"connect_mode": "identifier",
				},
			}, nil
		}
		// fall through to scan-based discovery when direct identifier connect fails
	}

	address, name, err = b.scanForDevice(ctx, printer)
	if err != nil {
		return nil, err
	}
	connectMode = "scan"

	device, err := b.adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address.String(), err)
	}

	return &connection{
		device: device,
		meta: map[string]any{
			"address":      address.String(),
			"matched_name": name,
			"connect_mode": connectMode,
		},
	}, nil
}

func (b *Backend) Scan(ctx context.Context) ([]transport.ScanResult, error) {
	if err := b.enable(); err != nil {
		return nil, fmt.Errorf("enable ble adapter: %w", err)
	}

	ctx, cancel := withDefaultTimeout(ctx, defaultScanTimeout)
	defer cancel()

	results := make(map[string]transport.ScanResult)
	errCh := make(chan error, 1)

	go func() {
		err := b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			key := result.Address.String()
			results[key] = transport.ScanResult{
				Address: key,
				Name:    result.LocalName(),
				RSSI:    result.RSSI,
			}
		})
		if err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	<-ctx.Done()
	_ = b.adapter.StopScan()
	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("scan ble: %w", err)
	}

	out := make([]transport.ScanResult, 0, len(results))
	for _, result := range results {
		out = append(out, result)
	}
	return out, nil
}

func (b *Backend) ScanStream(ctx context.Context) (<-chan transport.ScanResult, <-chan error, error) {
	if err := b.enable(); err != nil {
		return nil, nil, fmt.Errorf("enable ble adapter: %w", err)
	}

	results := make(chan transport.ScanResult, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(results)
		defer close(errs)
		done := make(chan error, 1)
		go func() {
			done <- b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
				select {
				case results <- transport.ScanResult{Address: result.Address.String(), Name: result.LocalName(), RSSI: result.RSSI}:
				case <-ctx.Done():
				}
			})
		}()

		select {
		case <-ctx.Done():
			_ = b.adapter.StopScan()
			if err := <-done; err != nil {
				errs <- fmt.Errorf("scan ble: %w", err)
			}
		case err := <-done:
			if err != nil {
				errs <- fmt.Errorf("scan ble: %w", err)
			}
		}
	}()

	return results, errs, nil
}

func (b *Backend) enable() error {
	b.enableOnce.Do(func() {
		b.enableErr = b.adapter.Enable()
	})
	return b.enableErr
}

func (b *Backend) scanForDevice(ctx context.Context, printer config.PrinterProfile) (bluetooth.Address, string, error) {
	ctx, cancel := withDefaultTimeout(ctx, defaultScanTimeout)
	defer cancel()

	type foundDevice struct {
		address bluetooth.Address
		name    string
	}
	foundCh := make(chan foundDevice, 1)
	errCh := make(chan error, 1)

	go func() {
		err := b.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			if !matchesPrinter(printer, result) {
				return
			}
			select {
			case foundCh <- foundDevice{address: result.Address, name: result.LocalName()}:
			default:
			}
			_ = adapter.StopScan()
		})
		if err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
		close(errCh)
	}()

	select {
	case found := <-foundCh:
		return found.address, found.name, nil
	case <-ctx.Done():
		_ = b.adapter.StopScan()
		return bluetooth.Address{}, "", fmt.Errorf("scan for printer %q timed out", printer.Name)
	case err := <-errCh:
		if err == nil {
			return bluetooth.Address{}, "", errors.New("scan ended before matching printer was found")
		}
		return bluetooth.Address{}, "", fmt.Errorf("scan for printer %q: %w", printer.Name, err)
	}
}

type connection struct {
	device         bluetooth.Device
	meta           map[string]any
	niimbotService bluetooth.DeviceService
	niimbotChar    bluetooth.DeviceCharacteristic
	notifyMu       sync.Mutex
	notifications  []notificationEntry
	notifyEnabled  bool
	deviceType     *int
}

type notificationEntry struct {
	Raw    []byte         `json:"raw"`
	Parsed map[string]any `json:"parsed,omitempty"`
}

func (c *connection) Print(ctx context.Context, printer config.PrinterProfile, job transport.Job) error {
	if err := c.ensureNiimbotCharacteristic(ctx); err != nil {
		return err
	}
	if err := c.ensureNotifications(); err != nil {
		return err
	}
	deviceType, err := c.deviceTypeID()
	if err == nil {
		c.meta["device_type"] = deviceType
	}
	switch strings.ToUpper(printer.Model) {
	case "D110":
		if err == nil && deviceType == 2320 {
			return c.printD110MV4(printer, job)
		}
		return c.printD110(printer, job)
	case "B1":
		return c.printB1(printer, job)
	default:
		return fmt.Errorf("niimbot packet session started but print task is not implemented yet for model %s", printer.Model)
	}
}

func (c *connection) Probe(_ context.Context) error {
	services, err := c.device.DiscoverServices(nil)
	if err != nil {
		return fmt.Errorf("discover services: %w", err)
	}

	discoveredServices := make([]map[string]any, 0, len(services))
	for _, service := range services {
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			discoveredServices = append(discoveredServices, map[string]any{
				"uuid":  service.UUID().String(),
				"error": err.Error(),
			})
			continue
		}

		characteristics := make([]map[string]any, 0, len(chars))
		for _, char := range chars {
			entry := map[string]any{
				"uuid": char.UUID().String(),
			}
			if mtu, err := char.GetMTU(); err == nil {
				entry["mtu"] = mtu
			}
			characteristics = append(characteristics, entry)
		}

		discoveredServices = append(discoveredServices, map[string]any{
			"uuid":            service.UUID().String(),
			"characteristics": characteristics,
		})
	}

	c.meta["gatt"] = map[string]any{
		"service_count": len(discoveredServices),
		"services":      discoveredServices,
	}
	if err := c.selectKnownNiimbotCharacteristic(services); err == nil {
		c.meta["niimbot"] = map[string]any{
			"service_uuid":        c.niimbotService.UUID().String(),
			"characteristic_uuid": c.niimbotChar.UUID().String(),
		}
	}
	return nil
}

func (c *connection) Close() error {
	return c.device.Disconnect()
}

func (c *connection) Metadata() map[string]any {
	return c.meta
}

func matchesPrinter(printer config.PrinterProfile, result bluetooth.ScanResult) bool {
	if printer.Identifier != "" && strings.EqualFold(result.Address.String(), printer.Identifier) {
		return true
	}
	if printer.Address != "" && strings.EqualFold(result.Address.String(), printer.Address) {
		return true
	}
	if printer.DeviceName != "" && result.LocalName() == printer.DeviceName {
		return true
	}
	return false
}

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *connection) ensureNiimbotCharacteristic(_ context.Context) error {
	services, err := c.device.DiscoverServices(nil)
	if err != nil {
		return fmt.Errorf("discover services: %w", err)
	}
	if err := c.selectKnownNiimbotCharacteristic(services); err != nil {
		return err
	}
	c.meta["niimbot"] = map[string]any{
		"service_uuid":        c.niimbotService.UUID().String(),
		"characteristic_uuid": c.niimbotChar.UUID().String(),
	}
	return nil
}

func (c *connection) ensureNotifications() error {
	if c.notifyEnabled {
		return nil
	}
	notificationCount := 0
	if err := c.niimbotChar.EnableNotifications(func(buf []byte) {
		entry := notificationEntry{Raw: append([]byte(nil), buf...)}
		if packet, err := niimbot.ParseFramedPacket(buf); err == nil {
			entry.Parsed = map[string]any{
				"command": packet.Command,
				"length":  packet.Length,
				"payload": append([]byte(nil), packet.Payload...),
			}
			if packet.Command == niimbot.CmdPrintStatus+16 && len(packet.Payload) >= 4 {
				entry.Parsed["status"] = map[string]any{
					"page":           int(packet.Payload[0])<<8 | int(packet.Payload[1]),
					"print_progress": int(packet.Payload[2]),
					"feed_progress":  int(packet.Payload[3]),
				}
			}
		}

		c.notifyMu.Lock()
		c.notifications = append(c.notifications, entry)
		if len(c.notifications) > 50 {
			c.notifications = c.notifications[len(c.notifications)-50:]
		}
		recent := append([]notificationEntry(nil), c.notifications...)
		c.notifyMu.Unlock()

		notificationCount++
		c.meta["niimbot_notifications"] = map[string]any{
			"count":       notificationCount,
			"last_packet": append([]byte(nil), buf...),
			"recent":      recent,
		}
	}); err != nil {
		return err
	}
	c.notifyEnabled = true
	c.meta["niimbot_notify_enabled"] = true
	return nil
}

func (c *connection) selectKnownNiimbotCharacteristic(services []bluetooth.DeviceService) error {
	for _, service := range services {
		if !strings.EqualFold(service.UUID().String(), niimbot.ServiceUUID) {
			continue
		}
		chars, err := service.DiscoverCharacteristics(nil)
		if err != nil {
			return fmt.Errorf("discover niimbot characteristics: %w", err)
		}
		for _, char := range chars {
			if strings.EqualFold(char.UUID().String(), niimbot.CharacteristicUUID) {
				c.niimbotService = service
				c.niimbotChar = char
				return nil
			}
		}
		return fmt.Errorf("niimbot characteristic %s not found", niimbot.CharacteristicUUID)
	}
	return fmt.Errorf("niimbot service %s not found", niimbot.ServiceUUID)
}

func (c *connection) printD110(printer config.PrinterProfile, job transport.Job) error {
	d110job, err := niimbot.PrepareD110Job(job.Rendered, printer.Defaults.Density)
	if err != nil {
		return err
	}

	setupPackets := []struct {
		name string
		data []byte
		resp byte
	}{
		{name: "set_density", data: niimbot.SetDensityPacket(d110job.Density), resp: niimbot.CmdSetDensity + 16},
		{name: "set_label_type", data: niimbot.SetLabelTypePacket(d110job.LabelType), resp: niimbot.CmdSetLabelType + 16},
		{name: "print_start", data: niimbot.PrintStartPacket(), resp: niimbot.CmdPrintStart + 1},
		{name: "print_clear", data: niimbot.PrintClearPacket(), resp: niimbot.CmdPrintClear + 16},
		{name: "page_start", data: niimbot.PageStartPacket(), resp: niimbot.CmdPageStart + 1},
		{name: "page_size", data: niimbot.SetPageSizePacket(d110job.HeightPx, d110job.WidthPx), resp: niimbot.CmdSetPageSize + 1},
		{name: "print_quantity", data: niimbot.PrintQuantityPacket(job.Copies), resp: niimbot.CmdPrintQuantity + 1},
	}
	for _, packet := range setupPackets {
		if _, err := c.transceive(packet.name, packet.data, packet.resp, 2*time.Second); err != nil {
			return err
		}
	}

	for pos, row := range d110job.Rows {
		if isEmptyRow(row) {
			if err := c.writePacket("empty_row", niimbot.EmptyRowPacket(pos, 1)); err != nil {
				return err
			}
			continue
		}
		blackPixels := niimbot.CountBlackPixelsForDebug(row)
		packetName := "bitmap_row"
		packet := niimbot.BitmapRowPacket(pos, row)
		if blackPixels <= 6 {
			packetName = "bitmap_row_indexed"
			packet = niimbot.BitmapRowIndexedPacket(pos, row)
		}
		if err := c.writePacket(packetName, packet); err != nil {
			return err
		}
	}

	if _, err := c.transceive("page_end", niimbot.PageEndPacket(), niimbot.CmdPageEnd+1, 2*time.Second); err != nil {
		return err
	}
	if err := c.waitForPrintStatus(); err != nil {
		return err
	}
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

	c.meta["d110_job"] = map[string]any{
		"rows":            len(d110job.Rows),
		"row_bytes":       d110job.WidthPx / 8,
		"density":         d110job.Density,
		"label_type":      d110job.LabelType,
		"empty_row_runs":  false,
		"oriented_width":  d110job.WidthPx,
		"oriented_height": d110job.HeightPx,
	}

	return nil
}

func (c *connection) printD110MV4(printer config.PrinterProfile, job transport.Job) error {
	d110job, err := niimbot.PrepareD110Job(job.Rendered, printer.Defaults.Density)
	if err != nil {
		return err
	}

	setupPackets := []struct {
		name string
		data []byte
		resp byte
	}{
		{name: "set_density", data: niimbot.SetDensityPacket(d110job.Density), resp: niimbot.CmdSetDensity + 16},
		{name: "set_label_type", data: niimbot.SetLabelTypePacket(d110job.LabelType), resp: niimbot.CmdSetLabelType + 16},
		{name: "print_start_v4", data: niimbot.PrintStartV4Packet(1, 0, 1), resp: niimbot.CmdPrintStart + 1},
	}
	for _, packet := range setupPackets {
		if _, err := c.transceive(packet.name, packet.data, packet.resp, 2*time.Second); err != nil {
			return err
		}
	}
	if err := c.writePacket("print_status_one_way", niimbot.PrintStatusPacket()); err != nil {
		return err
	}
	if _, err := c.transceive("page_size_v4", niimbot.SetPageSizeV4Packet(d110job.HeightPx, d110job.WidthPx, job.Copies), niimbot.CmdSetPageSize+1, 2*time.Second); err != nil {
		return err
	}

	for pos, row := range d110job.Rows {
		if isEmptyRow(row) {
			if err := c.writePacket("empty_row", niimbot.EmptyRowPacket(pos, 1)); err != nil {
				return err
			}
			continue
		}
		blackPixels := niimbot.CountBlackPixelsForDebug(row)
		packetName := "bitmap_row"
		packet := niimbot.BitmapRowPacket(pos, row)
		if blackPixels <= 6 {
			packetName = "bitmap_row_indexed"
			packet = niimbot.BitmapRowIndexedPacket(pos, row)
		}
		if err := c.writePacket(packetName, packet); err != nil {
			return err
		}
	}

	if _, err := c.transceive("page_end", niimbot.PageEndPacket(), niimbot.CmdPageEnd+1, 2*time.Second); err != nil {
		return err
	}
	if err := c.waitForPrintStatus(); err != nil {
		return err
	}
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
	_ = c.writePacket("heartbeat_one_way", niimbot.HeartbeatPacket())

	c.meta["d110_job"] = map[string]any{
		"rows":            len(d110job.Rows),
		"row_bytes":       d110job.WidthPx / 8,
		"density":         d110job.Density,
		"label_type":      d110job.LabelType,
		"empty_row_runs":  false,
		"oriented_width":  d110job.WidthPx,
		"oriented_height": d110job.HeightPx,
		"variant":         "d110_m_v4",
	}
	return nil
}

func (c *connection) printB1(printer config.PrinterProfile, job transport.Job) error {
	rasterJob, err := niimbot.PrepareRasterJob(job.Rendered)
	if err != nil {
		return err
	}
	density := printer.Defaults.Density
	if density <= 0 {
		density = 3
	}
	setupPackets := []struct {
		name string
		data []byte
		resp byte
	}{
		{name: "set_density", data: niimbot.SetDensityPacket(byte(density)), resp: niimbot.CmdSetDensity + 16},
		{name: "set_label_type", data: niimbot.SetLabelTypePacket(0x01), resp: niimbot.CmdSetLabelType + 16},
		{name: "print_start_pages", data: niimbot.PrintStartPagesPacket(1, 0), resp: niimbot.CmdPrintStart + 1},
		{name: "page_start", data: niimbot.PageStartPacket(), resp: niimbot.CmdPageStart + 1},
		{name: "page_size_pages", data: niimbot.SetPageSizePagesPacket(rasterJob.HeightPx, rasterJob.WidthPx, job.Copies), resp: niimbot.CmdSetPageSize + 1},
	}
	for _, packet := range setupPackets {
		if _, err := c.transceive(packet.name, packet.data, packet.resp, 2*time.Second); err != nil {
			return err
		}
	}
	for pos, row := range rasterJob.Rows {
		if isEmptyRow(row) {
			if err := c.writePacket("empty_row", niimbot.EmptyRowPacket(pos, 1)); err != nil {
				return err
			}
			continue
		}
		blackPixels := niimbot.CountBlackPixelsForDebug(row)
		packetName := "bitmap_row"
		packet := niimbot.BitmapRowPacket(pos, row)
		if blackPixels <= 6 {
			packetName = "bitmap_row_indexed"
			packet = niimbot.BitmapRowIndexedPacket(pos, row)
		}
		if err := c.writePacket(packetName, packet); err != nil {
			return err
		}
	}
	if _, err := c.transceive("page_end", niimbot.PageEndPacket(), niimbot.CmdPageEnd+1, 2*time.Second); err != nil {
		return err
	}
	if err := c.waitForPrintStatus(); err != nil {
		return err
	}
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
	c.meta["b1_job"] = map[string]any{
		"rows":      len(rasterJob.Rows),
		"row_bytes": len(rasterJob.Rows[0]),
		"density":   density,
		"width_px":  rasterJob.WidthPx,
		"height_px": rasterJob.HeightPx,
	}
	return nil
}

func (c *connection) waitForPrintStatus() error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		payload, err := c.transceive("print_status", niimbot.PrintStatusPacket(), niimbot.CmdPrintStatus+16, 5*time.Second)
		if err != nil {
			return err
		}
		if len(payload) >= 4 {
			status := map[string]any{
				"page":           int(payload[0])<<8 | int(payload[1]),
				"print_progress": int(payload[2]),
				"feed_progress":  int(payload[3]),
			}
			c.meta["last_print_status"] = status
			page, _ := status["page"].(int)
			printProgress, _ := status["print_progress"].(int)
			feedProgress, _ := status["feed_progress"].(int)
			if page >= 1 && printProgress >= 100 && feedProgress >= 100 {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for print completion")
}

func (c *connection) transceive(name string, packet []byte, respCmd byte, timeout time.Duration) ([]byte, error) {
	before := c.notificationCount()
	if err := c.writePacket(name, packet); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if payload, ok := c.findResponseSince(before, respCmd); ok {
			return payload, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for %s response 0x%02x", name, respCmd)
}

func (c *connection) deviceTypeID() (int, error) {
	if c.deviceType != nil {
		return *c.deviceType, nil
	}
	payload, err := c.transceive("get_info_device_type", niimbot.GetInfoPacket(0x08), 0x48, 2*time.Second)
	if err != nil {
		return 0, err
	}
	v := 0
	for _, b := range payload {
		v = (v << 8) | int(b)
	}
	c.deviceType = &v
	return v, nil
}

func (c *connection) notificationCount() int {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	return len(c.notifications)
}

func (c *connection) findResponseSince(start int, cmd byte) ([]byte, bool) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if start < 0 {
		start = 0
	}
	if start > len(c.notifications) {
		start = len(c.notifications)
	}
	for i := start; i < len(c.notifications); i++ {
		parsed := c.notifications[i].Parsed
		if parsed == nil {
			continue
		}
		got, ok := parsed["command"].(byte)
		if !ok {
			if f, ok := parsed["command"].(float64); ok {
				got = byte(f)
			} else if n, ok := parsed["command"].(int); ok {
				got = byte(n)
			} else {
				continue
			}
		}
		if got != cmd {
			continue
		}
		payload, ok := parsed["payload"].([]byte)
		if ok {
			return append([]byte(nil), payload...), true
		}
	}
	return nil, false
}

func (c *connection) writePacket(name string, packet []byte) error {
	if _, err := c.niimbotChar.WriteWithoutResponse(packet); err != nil {
		return fmt.Errorf("write %s packet: %w", name, err)
	}
	c.meta["last_packet"] = map[string]any{
		"name":  name,
		"bytes": append([]byte(nil), packet...),
	}
	time.Sleep(writeInterval)
	return nil
}

func isEmptyRow(row []byte) bool {
	for _, b := range row {
		if b != 0 {
			return false
		}
	}
	return true
}
