<h1>
<p align="center">
  <!-- <img src="assets/logo.svg" alt="niimtui logo" width="128"> LOGO HERE-->
  <br>niimtui
</h1>

<p align="center">
  Terminal-first label design and printing for Niimbot printers.
  <br />
  Design labels in a TUI, preview them, and print directly to supported devices.
  <br />
  <a href="#quick-start">Quick Start</a>
  ·
  <a href="#tui">TUI</a>
  ·
  <a href="#support">Support</a>
  ·
  <a href="#roadmap">Roadmap</a>
  ·
  <a href="#development">Development</a>
</p>

---

<div align="center">
  <video src="https://github.com/user-attachments/assets/205564aa-506e-4792-ae7e-b0abf91e681b" width="500" controls></video>
</div>

---


`niimtui` is a Go TUI for designing labels in the terminal and printing them to Niimbot printers over Bluetooth Low Energy.

It is built around the terminal workflow first: open the designer, compose a label, preview/export the rendered PNG, and print to a configured printer. The lower-level CLI and local HTTP service are still available for automation, debugging, and future Homebox integration.

- **terminal label designer** - mouse and keyboard editing for text, QR labels, sizing, positioning, grids, copy/paste, undo/redo, and saved design presets.
- **print what you preview** - exported previews use the same render path that feeds the printer pipeline.
- **local-first printing** - no vendor cloud or browser Bluetooth dependency; printing happens from your machine over BLE.
- **printer-aware layouts** - JSON printer profiles and label presets keep model, stock, shape, offsets, and defaults outside the label content.
- **Niimbot protocol work** - working macOS BLE paths for `D110_M` v4-class devices and `B1`, with model-specific print task selection.
- **automation-ready** - CLI commands and a local service exist alongside the TUI for scripts, probes, scans, previews, and future Homebox workflows.

## quick start

Run the default flow. If no default config exists, `niimtui` starts setup first; otherwise it opens the TUI.

```bash
go run ./cmd/niimtui
```

Run setup explicitly:

```bash
go run ./cmd/niimtui setup
```

Open the TUI with an existing config:

```bash
go run ./cmd/niimtui tui --config ./config.example.json
```

Open the TUI for an ad-hoc label size:

```bash
go run ./cmd/niimtui tui --width-mm 50 --height-mm 30
```

## tui

The TUI is the primary interface for the project.

- Design labels directly in a terminal canvas.
- Use mouse interactions for selection, movement, resizing, and layout work.
- Use keyboard shortcuts for fast editing, exporting, printing, copy/paste, undo/redo, and menu actions.
- Export PNG previews before printing.
- Print from the designer when a printer profile is configured.
- Use terminal image preview in supported terminals such as Kitty and Ghostty.

The designer works best in a large terminal window. Current minimum target size is roughly `140x30` cells.

## cli examples

Scan for nearby BLE devices:

```bash
go run ./cmd/niimtui scan --config ./config.example.json --transport ble
```

Probe a configured printer:

```bash
go run ./cmd/niimtui probe --config ./config.example.json --printer d110-desk
```

Render a preview without printing:

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

Print the same QR label:

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

## support

`niimtui` is beta software. The working hardware path today is macOS BLE with the printers below.

| OS | Transport | Printer | Status |
| --- | --- | --- | --- |
| macOS | BLE | `D110_M` v4-class | verified |
| macOS | BLE | `B1` | verified |
| macOS | Serial/USB | `all` | untested |
| Linux | BLE | pending | untested |
| Windows | BLE | pending | untested |
| SSH session | TUI | pending | untested |

See [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md) for the currently verified hardware, commands, and printer notes.

## homebox and service mode

The long-term integration target is a local `niimtui` service that can receive authenticated print requests from Homebox-style workflows while keeping rendering and printer-specific logic local.

The current service/CLI path supports QR text plus optional title/subtitle, resolves a configured printer and label preset, renders locally, and prints over BLE.

## roadmap

| Item | Status |
| --- | --- |
| Serial/USB support | ❌ |
| Full HTTPS Homebox integration | ❌ |
| Homebrew tap release (`brew install davidsanchez222/tap/niimtui`) | ❌ |
| Homebrew Core submission (`brew install niimtui`) | ❌ |
| Windows package manager publishing: winget, Scoop, Chocolatey | ❌ |
| Linux testing | ❌ |
| Windows testing | ❌ |
| SSH session testing | ❌ |
| BLE connectivity refactor and reliability improvements | ❌ |

More implementation detail lives in [`docs/ROADMAP.md`](./docs/ROADMAP.md).

## documentation

- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) - technical overview and design notes
- [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md) - verified setup and printer-specific commands
- [`docs/ROADMAP.md`](./docs/ROADMAP.md) - milestones and future work

## development

```bash
go build ./...
go test ./...
```

The project is written in Go and uses Bubble Tea/Lip Gloss for the TUI.
