# niimtui (beta)
<div align="center">
  <img width="360" alt="d110clidemo" src="https://github.com/user-attachments/assets/3196c342-9ff2-4710-bc47-cdceb10ce2d3" />
  <img width="244" height="400" alt="d110printingDemo2" src="https://github.com/user-attachments/assets/765fe6f6-bdcb-482d-b689-de1105338455" />
</div>

`niimtui` is a local CLI and HTTP service written for printing QR-driven labels to Niimbot printers over Bluetooth Low Energy written in Go.

Currently, it has only been verified using macOS BLE print paths for:

- `D110_M` v4-class devices
- `B1`

## Why

Printing to Niimbot printers from macOS can be awkward, especially when the practical options are tied to browser-specific BLE support or a web app workflow.

`niimtui` provides a simpler path: a local CLI and HTTP service that can send PNG label images directly to Niimbot printers over BLE.

## Features

- macOS BLE printing for Niimbot printers
- local QR generation from `qr.text`
- B1-first semantic label layouts for common stock sizes
- local CLI for print, scan, and probe workflows
- HTTP service mode for remote print requests
- browser-facing loopback CORS support for Homebox-style integrations
- JSON-configured printer profiles and label presets
- device-type-aware print task selection

## Current Support

| OS    | Transport | Printer         | Status   |
| ----- | --------- | --------------- | -------- |
| macOS | BLE       | D110_M v4-class | verified |
| macOS | BLE       | B1              | verified |

## Quick Start

Scan for nearby BLE devices:

```bash
go run ./cmd/niimtui scan --config ./config.example.json --transport ble
```

Probe a configured printer:

```bash
go run ./cmd/niimtui probe --config ./config.example.json --printer d110-desk
```

Print a QR label:

```bash
go run ./cmd/niimtui print \
  --config ./config.example.json \
  --printer b1-round \
  --preset b1-50x50-round \
  --layout qr-title-subtitle \
  --qr-text https://homebox.example/items/123 \
  --title "Garage Bin 4" \
  --subtitle "Top Shelf" \
  --preview-out ./preview.png
```

Generate the exact rendered print-job preview without printing:

```bash
go run ./cmd/niimtui print \
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

## How It Works

1. A caller provides QR text, optional human-readable label text, and a target printer profile.
2. `niimtui` resolves the preset, composes the label locally, prepares raster data, and selects the correct model-specific print task.
3. `niimtui` sends the print job over BLE and tracks printer status until completion.

When `--preview-out` is used, the saved PNG is the same rendered label image that would be sent into the printer path. Add `--no-print` to stop after writing the preview.

## Current Input Mode

The validated path today is text-driven QR label printing:

- input: `qr.text` plus optional title/subtitle
- output: BLE print job to the configured Niimbot printer

The current browser integration target is Homebox in a normal browser on the same Mac as `niimtui`, with the browser calling the local service directly.

## Documentation

- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md): full technical overview and design details
- [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md): verified hardware setup, working commands, and printer notes
- [`docs/ROADMAP.md`](./docs/ROADMAP.md): implementation milestones and future work

## Status

This project is in active development, but the direct macOS BLE image-print path is already working on real hardware.
