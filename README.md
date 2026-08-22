# niimcli (beta)
<div align="center">
  <img width="360" alt="d110clidemo" src="https://github.com/user-attachments/assets/3196c342-9ff2-4710-bc47-cdceb10ce2d3" />
  <img width="244" height="400" alt="d110printingDemo2" src="https://github.com/user-attachments/assets/765fe6f6-bdcb-482d-b689-de1105338455" />
</div>

`niimcli` is a local CLI and HTTP service written for printing PNG label images to Niimbot printers over Bluetooth Low Energy written in Go.

Currently, it has only been verified using macOS BLE print paths for:

- `D110_M` v4-class devices
- `B1`

## Demo

Demo links will live here:

- CLI demo video: coming soon
- Real printer demo video: coming soon

## Why

Printing to Niimbot printers from macOS can be awkward, especially when the practical options are tied to browser-specific BLE support or a web app workflow.

`niimcli` provides a simpler path: a local CLI and HTTP service that can send PNG label images directly to Niimbot printers over BLE.

## Features

- macOS BLE printing for Niimbot printers
- verified direct image printing for `D110_M` v4 and `B1`
- local CLI for print, scan, and probe workflows
- HTTP service mode for remote print requests
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
go run ./cmd/niimcli scan --config ./config.example.json --transport ble
```

Probe a configured printer:

```bash
go run ./cmd/niimcli probe --config ./config.example.json --printer d110-desk
```

Print a full label image:

```bash
go run ./cmd/niimcli print \
  --config ./config.example.json \
  --printer d110-desk \
  --preset d110-12x40 \
  --image ./example.png \
  --preview-out ./preview.png
```

## How It Works

1. A caller provides a PNG label image and a target printer profile (device name and label size)
2. `niimcli` resolves the preset, prepares raster data, and selects the correct model-specific print task.
3. `niimcli` sends the print job over BLE and tracks printer status until completion.

## Current Input Mode

The validated path today is direct full-image printing:

- input: final PNG label image
- output: BLE print job to the configured Niimbot printer

Future composition modes such as QR-only input and service-side label layout are planned, but they are not the primary path yet.

## Documentation

- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md): full technical overview and design details
- [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md): verified hardware setup, working commands, and printer notes
- [`docs/ROADMAP.md`](./docs/ROADMAP.md): implementation milestones and future work

## Status

This project is in active development, but the direct macOS BLE image-print path is already working on real hardware.
