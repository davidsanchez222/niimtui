package ble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"niimtui/internal/config"
	"niimtui/internal/printtrace"
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
}

var defaultAdapterEnable struct {
	once sync.Once
	err  error
}

func New() *Backend {
	return &Backend{adapter: bluetooth.DefaultAdapter}
}

func (b *Backend) Connect(ctx context.Context, printer config.PrinterProfile) (transport.Connection, error) {
	printtrace.Mark(ctx, "BLE connect started")
	if err := b.enable(); err != nil {
		return nil, fmt.Errorf("enable ble adapter: %w", err)
	}

	var (
		address     bluetooth.Address
		name        string
		connectMode string
		err         error
	)

	// CoreBluetooth reconnects by identifier can produce a zero-value device after
	// a previous print session. Prefer rediscovering by advertisement when we have
	// a configured device name, then connect to the discovered peripheral.
	if printer.DeviceName != "" {
		address, name, err = b.scanForDevice(ctx, printer)
		printtrace.Mark(ctx, "BLE scan by name complete")
		if err != nil && ctx.Err() != nil {
			return nil, err
		}
		if err != nil && printer.Identifier == "" && printer.Address == "" {
			return nil, err
		}
		if err == nil {
			connectMode = "scan"
		}
	}

	if err != nil && printer.Identifier != "" {
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

	if connectMode == "" {
		address, name, err = b.scanForDevice(ctx, printer)
		printtrace.Mark(ctx, "BLE fallback scan complete")
		if err != nil {
			return nil, err
		}
		connectMode = "scan"
	}

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
	defaultAdapterEnable.once.Do(func() {
		defaultAdapterEnable.err = b.adapter.Enable()
	})
	return defaultAdapterEnable.err
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
	device          bluetooth.Device
	meta            map[string]any
	metaMu          sync.Mutex
	niimbotService  bluetooth.DeviceService
	niimbotChar     bluetooth.DeviceCharacteristic
	notifyMu        sync.Mutex
	notifications   []notificationEntry
	notificationSeq uint64
	notifyEnabled   bool
	deviceType      *int
}

type notificationEntry struct {
	seq    uint64
	Raw    []byte         `json:"raw"`
	Parsed map[string]any `json:"parsed,omitempty"`
}

func (c *connection) Prepare(ctx context.Context, _ config.PrinterProfile) error {
	printtrace.Mark(ctx, "BLE print preparation started")
	if !c.notifyEnabled {
		if err := c.ensureNiimbotCharacteristic(ctx); err != nil {
			return err
		}
		printtrace.Mark(ctx, "GATT discovery complete")
		if err := c.ensureNotifications(); err != nil {
			return err
		}
		printtrace.Mark(ctx, "BLE notifications enabled")
	}
	if c.deviceType == nil {
		printtrace.Mark(ctx, "first BLE print write starting (device type request)")
		deviceType, err := c.deviceTypeID()
		printtrace.Mark(ctx, "device type lookup complete")
		if err == nil {
			c.setMeta("device_type", deviceType)
		}
	}
	return nil
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

	c.setMeta("gatt", map[string]any{
		"service_count": len(discoveredServices),
		"services":      discoveredServices,
	})
	if err := c.selectKnownNiimbotCharacteristic(services); err == nil {
		c.setMeta("niimbot", map[string]any{
			"service_uuid":        c.niimbotService.UUID().String(),
			"characteristic_uuid": c.niimbotChar.UUID().String(),
		})
	}
	return nil
}

func (c *connection) Close() error {
	return c.device.Disconnect()
}

func (c *connection) Metadata() map[string]any {
	c.metaMu.Lock()
	defer c.metaMu.Unlock()
	copyMeta := make(map[string]any, len(c.meta))
	for key, value := range c.meta {
		copyMeta[key] = value
	}
	return copyMeta
}

func (c *connection) setMeta(key string, value any) {
	c.metaMu.Lock()
	defer c.metaMu.Unlock()
	c.meta[key] = value
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
	c.setMeta("niimbot", map[string]any{
		"service_uuid":        c.niimbotService.UUID().String(),
		"characteristic_uuid": c.niimbotChar.UUID().String(),
	})
	return nil
}

func (c *connection) ensureNotifications() error {
	if c.notifyEnabled {
		return nil
	}
	if err := c.niimbotChar.EnableNotifications(func(buf []byte) {
		c.recordNotification(buf)
	}); err != nil {
		return err
	}
	c.notifyEnabled = true
	c.setMeta("niimbot_notify_enabled", true)
	return nil
}

func (c *connection) recordNotification(buf []byte) {
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
	c.notificationSeq++
	entry.seq = c.notificationSeq
	c.notifications = append(c.notifications, entry)
	if len(c.notifications) > 50 {
		c.notifications = c.notifications[len(c.notifications)-50:]
	}
	recent := append([]notificationEntry(nil), c.notifications...)
	count := c.notificationSeq
	c.notifyMu.Unlock()

	c.setMeta("niimbot_notifications", map[string]any{
		"count":       count,
		"last_packet": append([]byte(nil), buf...),
		"recent":      recent,
	})
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
			c.setMeta("last_print_status", status)
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

func (c *connection) notificationCount() uint64 {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	return c.notificationSeq
}

func (c *connection) findResponseSince(start uint64, cmd byte) ([]byte, bool) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	for _, entry := range c.notifications {
		if entry.seq <= start {
			continue
		}
		parsed := entry.Parsed
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
	c.setMeta("last_packet", map[string]any{
		"name":  name,
		"bytes": append([]byte(nil), packet...),
	})
	time.Sleep(writeInterval)
	return nil
}

func (c *connection) writeRows(ctx context.Context, rows [][]byte) error {
	packets := 0
	err := sendRows(rows, func(name string, data []byte) error {
		packets++
		return c.writePacket(name, data)
	})
	printtrace.Mark(ctx, fmt.Sprintf("raster sent: %d rows in %d packets", len(rows), packets))
	return err
}

// sendRows preserves the first row position while encoding runs of equal
// rows. The repeat field is a single byte, so longer runs must be split.
func sendRows(rows [][]byte, write func(string, []byte) error) error {
	for pos := 0; pos < len(rows); {
		row := rows[pos]
		repeat := 1
		for repeat < 255 && pos+repeat < len(rows) && bytes.Equal(row, rows[pos+repeat]) {
			repeat++
		}
		var name string
		var packet []byte
		if isEmptyRow(row) {
			name = "empty_row"
			packet = niimbot.EmptyRowPacket(pos, repeat)
		} else if niimbot.CountBlackPixelsForDebug(row) <= 6 {
			name = "bitmap_row_indexed"
			packet = niimbot.BitmapRowIndexedPacketRepeated(pos, repeat, row)
		} else {
			name = "bitmap_row"
			packet = niimbot.BitmapRowPacketRepeated(pos, repeat, row)
		}
		if err := write(name, packet); err != nil {
			return err
		}
		pos += repeat
	}
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
