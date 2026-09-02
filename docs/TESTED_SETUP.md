# Tested Setup

This document captures the currently verified local setup and the commands that have been tested successfully.

## Verified Environment

- OS: macOS
- Transport: BLE
- Config format: JSON
- Input mode: `qr.text` plus optional title/subtitle

## Verified Printers

### D110_M

- Profile: `d110-desk`
- Advertised name: `D110_M-H913040249`
- CoreBluetooth identifier: `ea88cc93-a2a2-8287-1f08-18cd0a81649b`
- Detected device type: `2320`
- Selected task: `d110_m_v4`

Notes:

- This printer must not use the classic `D110` print path.
- It requires the `D110_M v4` task sequence selected by runtime device type.

### B1

- Profile: `b1-round`
- Advertised name: `B1-I427031488`
- CoreBluetooth identifier: `e6bc3bef-5a60-3bc7-ffab-50edc0e9f122`
- Detected device type: `4096`
- Selected task: `B1`

## Config Files

Useful files in the repo:

- `config.example.json`: example config with working local names and identifiers
- `config.sample.local.json`: sample local config for direct use as a starting point

Before using a config on another machine or with another printer, update:

- `server.auth_token`
- `printers[].device_name`
- `printers[].identifier`

## Working Commands

### Scan For BLE Devices

```bash
go run ./cmd/niimcli scan --config ./config.example.json --transport ble
```

### Probe D110_M

```bash
go run ./cmd/niimcli probe --config ./config.example.json --printer d110-desk
```

### Probe B1

```bash
go run ./cmd/niimcli probe --config ./config.example.json --printer b1-round
```

### Print On D110_M

```bash
go run ./cmd/niimcli print \
  --config ./config.example.json \
  --printer d110-desk \
  --preset d110-12x40 \
  --layout qr-only \
  --qr-text https://homebox.example/items/123 \
  --preview-out ./preview.png
```

### Preview Without Printing

```bash
go run ./cmd/niimcli print \
  --config ./config.example.json \
  --printer b1-round \
  --preset b1-50x50-round \
  --layout qr-title-subtitle \
  --qr-text https://homebox.example/items/123 \
  --title "Garage Bin 4" \
  --subtitle "Top Shelf" \
  --preview-out ./preview.png \
  --no-print
```

### Print On B1

```bash
go run ./cmd/niimcli print \
  --config ./config.example.json \
  --printer b1-round \
  --preset b1-50x50-round \
  --layout qr-title-subtitle \
  --qr-text https://homebox.example/items/123 \
  --title "Garage Bin 4" \
  --subtitle "Top Shelf" \
  --preview-out ./preview.png
```

## Current Input Mode

The currently validated path is:

- `qr.text` plus optional title/subtitle in
- printer-specific raster/protocol handling in `niimcli`
- BLE print out

This means the tested CLI and service path expects `niimcli` to generate the QR and compose the final label layout locally.

`--preview-out` writes that exact rendered print job to disk as a PNG. Add `--no-print` to stop before the BLE print step.

First-pass B1 layout heuristics:

- `50x30` rect: QR left, text right
- `50x50` round: QR centered, text below
- `50x80` rect: large QR above, text below

Later work should define how Homebox maps page data into `content.title` and `content.subtitle`.

## Debugging Notes

Useful outputs during debugging:

- `--preview-out ./preview.png` to inspect the rendered image passed into the printer path
- `probe` output to inspect GATT service and characteristic discovery
- print response JSON to inspect:
  - `meta.connection.device_type`
  - `meta.connection.d110_job` or `meta.connection.b1_job`
  - `meta.connection.last_print_status`
  - `meta.connection.niimbot_notifications`

## Known BLE Details

Known Niimbot BLE GATT path observed on both tested printers:

- service UUID: `e7810a71-73ae-499d-8c15-faa9aef0c3f2`
- characteristic UUID: `bef8d6c9-9c21-4c9e-b632-bd58c1009f9f`

## Validation

The following commands are expected to pass locally:

```bash
go build ./...
go test ./...
```
