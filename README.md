# niimtui

<p align="center">
  <strong>Terminal-first label design and printing for Niimbot printers.</strong>
</p>

<p align="center">
  <a href="#quick-start">quick start</a> · <a href="#tui">tui</a> · <a href="#support">support</a> · <a href="#roadmap">roadmap</a> · <a href="#development">development</a>
</p>

<p align="center">
  Logo placeholder: <code>assets/logo.svg</code> coming soon.
</p>

---
<table>
  <tr>
    <td align="center">
      <strong>Demo</strong>
    </td>
    <td align="center">
      <strong>Result</strong>
    </td>
  </tr>
  <tr>
    <td>
      <video src="https://github.com/user-attachments/assets/f5d55ace-b483-4972-8820-57423abd367e" width="400" controls></video>
    </td>
    <td>
      <img width="469" height="515" alt="actualPrint3" src="https://github.com/user-attachments/assets/a9fc0a1a-35db-465e-bb0f-a269b59b3b58" />
    </td>
  </tr>
</table>
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
| Serial/USB support | pending |
| Full HTTPS Homebox integration | pending |
| Package manager publishing: Homebrew, Chocolatey, apt, etc. | pending |
| Linux testing | pending |
| Windows testing | pending |
| SSH session testing | pending |
| BLE connectivity refactor and reliability improvements | pending |

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
