# niimtui (beta)

`niimtui` is a local CLI and HTTP or HTTPS service for printing to Niimbot label printers over Bluetooth Low Energy.

It is designed for workflows where another tool, script, or automation needs to send a label image or print request to a nearby machine that has access to a Niimbot printer.

## Quickstart Flow

Typical usage:

```text
Browser UI or automation
-> POST /print
-> niimtui resolves printer + preset
-> niimtui generates QR and composes the final label
-> niimtui prints over BLE
```

The current working path sends QR text plus optional label text. `niimtui` handles local QR generation, preset-aware composition, and printer-specific transport/protocol details.

## Status

`niimtui` has working end-to-end macOS BLE print paths for D110_M v4-class devices and B1 using direct PNG image input.

The next implementation target is to expand the service contract cleanly for future composition modes and continue broadening model support.

## Planned Support

| Area             | Version 1 Target              |
| ---------------- | ----------------------------- |
| Operating system | macOS                         |
| Transport        | BLE                           |
| Printers         | D110_M v4 working, B1 working |
| Service API      | HTTPS                         |
| Input            | QR text plus optional text    |
| Config format    | JSON                          |

## Features

- local CLI for direct printing and debugging
- HTTPS service mode for remote print requests
- BLE-first support for Niimbot printers
- named printer profiles for selecting between configured printers
- semantic QR-first label requests
- model-aware BLE protocol handling
- device-type-aware print task selection
- service-side QR composition with preset-aware layouts
- JSON configuration for printers, presets, and service settings

## Design Principles

- keep printer-specific logic out of upstream applications
- select printers through named local profiles
- render final labels inside the service, not the caller or Homebox backend
- start with one narrow BLE path and expand after it is reliable
- keep the external API simple and stable

## Use Cases

`niimtui` is intended for setups where label printing should be handled by a dedicated local service instead of embedding printer logic into another application.

Examples:

- sending print requests from browser automation
- triggering labels from inventory or asset systems
- running a small print service on a laptop or desktop near the printer
- standardizing label rendering across multiple Niimbot models and label shapes

Internally, `niimtui` resolves the target printer and label preset, renders or accepts the final label image, converts it to printer-ready raster data, selects the correct model-specific print task, and sends it to the printer over BLE.

## Initial Scope

Version 1 focuses on one narrow, reliable path:

- macOS
- BLE only
- D110_M v4-class devices and B1
- HTTPS service mode via `niimtui serve`
- synchronous `POST /print`
- QR text input
- service-rendered composition
- browser-to-loopback integration first
- JSON configuration for printer profiles and label presets

## Non-Goals

The first version does not target:

- browser-side direct BLE printing
- MQTT integration
- RFCOMM or USB transports
- support for many printer brands
- advanced label designer UI
- async job queueing

## Commands

Planned commands:

```text
niimtui serve --config /path/to/config.json
niimtui print --printer d110-desk --preset d110-12x40 --image ./label.png
niimtui probe --printer d110-desk
niimtui scan --transport ble
niimtui printers
niimtui presets
```

## Installation

Installation instructions will be added once the first runnable version exists.

The initial release is expected to ship as a single local binary with JSON-based configuration.

## Service API

The first service mode is a small authenticated HTTPS API.

Planned endpoints:

- `GET /health`
- `GET /printers`
- `GET /presets`
- `POST /probe`
- `POST /render-preview`
- `POST /print`

The initial `POST /print` flow is synchronous so the caller gets immediate success or failure.

## Print Request Model

The first request format is JSON and sends:

- a named printer selector
- a label preset that includes device and label type (ex: d110-12x40mm)
- a QR text payload
- optional text fields
- optional print options such as copy count

Example:

```json
{
  "printer": {
    "selector": "b1-round"
  },
  "label": {
    "preset": "round-40mm",
    "layout": "full-image"
  },
  "qr": {
    "text": "https://homebox.example/items/123"
  },
  "content": {
    "title": "Box 42",
    "subtitle": "Shelf A3"
  },
  "options": {
    "copies": 1
  }
}
```

The first version requires `qr.text`. Text fields are optional and are used for human-readable label content next to or below the QR depending on the preset.

## Printer Profiles

Requests should select printers by named profile rather than raw BLE details.

Example profile names:

- `d110-desk`
- `b1-round`
- `b1-wide`

Profiles keep device names, defaults, and model-specific settings in local config.

## JSON Configuration

The first version uses JSON for service config.

Configuration should define:

- server settings
- auth token
- TLS settings if enabled
- printer profiles
- label presets

### Printer Profiles

Each printer profile should describe one selectable printer target.

Recommended fields:

- `name`
- `model`
- `transport`
- `device_name`
- `identifier`
- `default_preset`

Optional fields can include model defaults such as density or rotation.

### Label Presets

Each preset should describe the target label stock and default layout assumptions.

Recommended fields:

- `name`
- `width_mm`
- `height_mm`
- `shape`
- `layout`
- `margins_mm`

### Example Config

Example shape:

```json
{
  "server": {
    "listen": "100.x.y.z:8443",
    "auth_token": "change-me"
  },
  "printers": [
    {
      "name": "d110-desk",
      "model": "D110",
      "transport": "ble",
      "device_name": "D110_M-H913040249",
      "default_preset": "d110-12x40"
    },
    {
      "name": "b1-round",
      "model": "B1",
      "transport": "ble",
      "device_name": "B1-ABC123",
      "default_preset": "round-40mm"
    }
  ],
  "presets": [
    {
      "name": "d110-12x40",
      "width_mm": 40,
      "height_mm": 12,
      "shape": "rect",
      "layout": "qr-only",
      "margins_mm": 1
    },
    {
      "name": "round-40mm",
      "width_mm": 40,
      "height_mm": 40,
      "shape": "round",
      "layout": "qr-only",
      "margins_mm": 2
    }
  ]
}
```

## Rendering Strategy

The current direct-print path accepts a full PNG label image.

Future composition modes can allow `niimtui` to build the final output based on:

- printer model
- preset dimensions
- label shape
- margins
- optional title and subtitle
- orientation and print direction rules

Initial layout plan:

- `qr-only`
- `qr-title`
- `qr-title-subtitle`

The current implementation is text-driven and generates the QR inside `niimtui`. The first layout heuristics are tuned for known B1 stock sizes, with `50x30` using QR-left/text-right, `50x50` round using centered QR with text below, and `50x80` using a large QR above a text block.

## Browser Integration

When Homebox is remote but viewed in a browser on the same Mac as the printer, the browser calls the local `niimtui` service directly.

- Homebox backend stores `niimtui_base_url` as a browser-side setting
- the browser resolves `http://127.0.0.1:8443` or `https://127.0.0.1:8443`
- `niimtui` enforces an origin allowlist for browser requests
- requests without an `Origin` header remain valid for local tools such as `curl`

Later work should define how Homebox page data maps into `content.title` and `content.subtitle` for different page types.

## Model Notes

Some printers that appear as `D110` at the UX level actually require a different print task at runtime.

For example, `D110_M` devices with device type `2320` use a different `v4` print sequence than classic `D110` devices. `niimtui` detects that device type and selects the matching task automatically.

## Security

The service should be deployed behind normal network controls and still require authentication.

Version 1 should include:

- bearer token authentication
- request size limits
- PNG-only input validation
- metadata-focused logging instead of raw image payload logging
- configurable TLS support

## Roadmap

See [`ROADMAP.md`](./ROADMAP.md) for milestone sequencing, API implementation order, and the first vertical slice.

## Additional Docs

- [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md): verified hardware setup, working commands, and printer-specific notes
