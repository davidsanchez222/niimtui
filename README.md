<div align="center">
  <!-- <img src="assets/logo.svg" alt="niimtui logo" width="128"> LOGO HERE-->
  <h1>niimtui</h1>
  <p>
    <a href="https://github.com/davidsanchez222/niimtui/releases/latest">
      <img
        alt="Latest release"
        src="https://img.shields.io/github/v/release/davidsanchez222/niimtui?style=for-the-badge&logo=starship&color=C9CBFF&logoColor=D9E0EE&labelColor=302D41&include_prerelease&sort=semver"
      />
    </a>
    <a href="https://github.com/davidsanchez222/niimtui/stargazers">
      <img
        alt="Stars"
        src="https://img.shields.io/github/stars/davidsanchez222/niimtui?style=for-the-badge&logo=starship&color=c69ff5&logoColor=D9E0EE&labelColor=302D41"
      />
    </a>
    <a href="https://github.com/davidsanchez222/niimtui/issues">
      <img
        alt="Issues"
        src="https://img.shields.io/github/issues/davidsanchez222/niimtui?style=for-the-badge&logo=bilibili&color=F5E0DC&logoColor=D9E0EE&labelColor=302D41"
      />
    </a>
    <a href="https://github.com/davidsanchez222/niimtui">
      <img
        alt="Repo Size"
        src="https://img.shields.io/github/repo-size/davidsanchez222/niimtui?color=%23DDB6F2&label=SIZE&logo=codesandbox&style=for-the-badge&logoColor=D9E0EE&labelColor=302D41"
      />
    </a>
    <a href="https://github.com/davidsanchez222/niimtui/blob/main/LICENSE">
      <img
        alt="License"
        src="https://img.shields.io/badge/License-MIT-ee999f?style=for-the-badge&logo=starship&logoColor=D9E0EE&labelColor=302D41"
      />
    </a>
    <img
      alt="Go 1.25 Required"
      src="https://img.shields.io/badge/Go-1.25%2B-8bd5ca?style=for-the-badge&logo=go&logoColor=D9E0EE&labelColor=302D41"
    />
    <img
      alt="macOS BLE verified"
      src="https://img.shields.io/badge/macOS-BLE%20verified-8aadf3?style=for-the-badge&logo=apple&logoColor=D9E0EE&labelColor=302D41"
    />
  </p>
  <p>
    <strong>terminal-first label design and printing for Niimbot printers</strong>
  </p>
  <p>
    design labels in a TUI, preview them, and print directly to supported devices.
  </p>
  <p>
    <a href="#quick-start">quick start</a>
    ·
    <a href="#tui">tui</a>
    ·
    <a href="#support">support</a>
    ·
    <a href="#roadmap">roadmap</a>
    ·
    <a href="#development">development</a>
  </p>
</div>

---

<div align="center">
  <video
    src="https://github.com/user-attachments/assets/8656b58b-12e7-4bf0-8572-b91d0f667313"
    width="500"
    loop
    muted
    >
  </video>
</div>

> [!NOTE]
> notice how the mouse wasn't used once to make the label above. only keystrokes! however, niimtui still has full mouse support
---

`niimtui` is a Go TUI for designing labels in the terminal and printing them to Niimbot printers over Bluetooth Low Energy.

It is built around the terminal workflow first: open the designer, compose a label, preview/export the rendered PNG, and print to a configured printer. The lower-level CLI and local HTTP service are still available for automation, debugging, and future Homebox integration.

- **terminal label designer** - mouse and keyboard editing for text, QR labels, sizing, positioning, grids, copy/paste, undo/redo, and saved design presets.
- **print what you preview** - exported previews use the same render path that feeds the printer pipeline.
- **local-first printing** - no vendor cloud or browser Bluetooth dependency; printing happens from your machine over BLE.
- **printer-aware layouts** - JSON printer profiles and label presets keep model, stock, shape, offsets, and defaults outside the label content.
- **Niimbot protocol work** - working macOS BLE paths for `D110_M` v4-class devices and `B1`, with model-specific print task selection.
- **automation-ready** - CLI commands and a local service exist alongside the TUI for scripts, probes, scans, previews, and future Homebox workflows.

## install

Packaged installs are being prepared for macOS first. After the first packaged release, the supported install path will be:

```bash
brew install davidsanchez222/tap/niimtui
```

Apple Silicon is the tested macOS platform; Intel builds will be published but remain untested until verified. Linux and Windows packages are planned, but will remain experimental until tested with real BLE printing setups. Until then, run from source with Go as shown below.

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
- Select a text element and press `F` to browse fonts. Scrolling previews the highlighted font on the canvas and in the terminal-image preview; Enter applies it. Esc leaves search mode, then closes the picker without changing the label.
- Export PNG previews before printing.
- Print from the designer when a printer profile is configured.
- Save designs with named text/QR bindings for reuse from the CLI. Select an element, press `b` to name its binding, and press `!` to require a new value for each CLI invocation. Press `s` to save it under **Custom** in the gallery. Auto Insert remains in the preferences menu (`m`).
- Saving under an existing name asks before overwriting; use `↑/↓` or `k/j` and Enter to choose Cancel or Overwrite, or press `n`/`y`. To delete a saved design, select it under Custom in the gallery and press `d`; the current canvas stays open.
- Press `Ctrl+T` (or click a tab header) to switch between the label designer and gallery. The gallery has collapsible **B1**, **D110**, and **Custom** groups. Use `↑/↓` or `k/j` to browse, `←/→` or `h/l` to fold groups, and Enter to edit. Starter text templates use font size 40 where they fit; small rolls offer suitable layouts instead. Previews use the canvas and, where supported, terminal images.
- On a saved design in the gallery, press `c` to see its runnable `niimtui print --design ...` command. The default config path is implicit; a custom config path is included when needed. Press `c` again to send the full command to an OSC 52-capable terminal clipboard, or drag across the command to highlight and copy it.
- Press `Tab` to focus the printer sidebar. It is a collapsible printer-and-roll tree: `↑/↓` or `k/j` moves through visible rows, `←/→` or `h/l` folds printers, Enter switches printers or selects a roll, `c` connects, `D` disconnects, `r` rescans, and Esc or Tab returns to the editor.
- Drag over rendered text outside the designer canvas to highlight and copy it, including gallery, menu, and popup text. Canvas dragging still moves and resizes elements. Terminal clipboard copying uses OSC 52 when supported.
- At startup, the TUI scans once for configured advertising BLE printers. It connects to the sole detected printer, or to the active printer when several are detected. If none is detected, it remains disconnected; a manual `c` in the focused printer sidebar can still try to connect. Auto-selection does not change your saved active printer.
- Use terminal image preview in supported terminals such as Kitty, Ghostty and WezTerm (WezTerm needs `enable_kitty_graphics = true`). On macOS terminals without inline images (Alacritty, Apple Terminal, iTerm2), each change instead rewrites one PNG and runs `open -a Preview` on it and returns focus to your terminal, so a single Preview window refreshes in place. Set `NIIMTUI_GRAPHICS=kitty` or `open` to force a mode, or `off` to disable the preview.

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

Discover saved designs and their binding names:

```bash
go run ./cmd/niimtui designs
```

Render a saved TUI design without connecting to a printer:

```bash
go run ./cmd/niimtui preview --design garage-bin \
  --set url=https://homebox.example/items/123 \
  --set title="Garage Bin 4" --out ./preview.png
```

Print it to the active printer and its installed default label roll:

```bash
go run ./cmd/niimtui print --design garage-bin \
  --set url=https://homebox.example/items/123 \
  --set title="Garage Bin 4"
```

For a one-off QR label, no saved design is needed:

```bash
go run ./cmd/niimtui print --qr https://homebox.example/items/123 \
  --title "Garage Bin 4" --subtitle "Top Shelf" --printer b1-round
```

`--config`, `--printer`, and `--preset` override defaults when needed. `--set name=value` replaces only that named text/QR element; omitted optional bindings keep their saved values. Required bindings must be provided even when a saved value exists. `print` rejects a design whose dimensions or shape don't match the selected preset; `preview` can still render it. `print`, `preview`, and `designs` return JSON on stdout; unsuccessful prints exit nonzero. The TUI's `s` command saves designs in your config file, so `config.example.json` by itself has no saved designs to list.

Previous CLI examples using `print --preview-out FILE --no-print` now use `preview --out FILE`. Use `--qr` in place of `--qr-text`; for quick labels, the presence of `--title` and `--subtitle` determines the layout.

## support

`niimtui` is beta software. The working hardware path today is macOS BLE with the printers below.

| OS          | Transport  | Printer           | Status   |
| ----------- | ---------- | ----------------- | -------- |
| macOS       | BLE        | `D110_M` v4-class | verified |
| macOS       | BLE        | `B1`              | verified |
| macOS       | Serial/USB | `all`             | untested |
| Linux       | BLE        | pending           | untested |
| Windows     | BLE        | pending           | untested |
| SSH session | TUI        | pending           | untested |

See [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md) for the currently verified hardware, commands, and printer notes.

## homebox and service mode

The long-term integration target is a local `niimtui` service that can receive authenticated print requests from Homebox-style workflows while keeping rendering and printer-specific logic local.

The current service/CLI path supports QR text plus optional title/subtitle, resolves a configured printer and label preset, renders locally, and prints over BLE.

## roadmap

| Item                                                              | Status |
| ----------------------------------------------------------------- | ------ |
| Serial/USB support                                                | ❌     |
| Full HTTPS Homebox integration                                    | ❌     |
| Homebrew tap release (`brew install davidsanchez222/tap/niimtui`) | ❌     |
| Homebrew Core submission (`brew install niimtui`)                 | ❌     |
| Windows package manager publishing: winget, then Scoop            | ❌     |
| Linux packaging: Arch (`niimtui-bin`), then apt `.deb`, then Nix  | ❌     |
| Linux testing                                                     | ❌     |
| Windows testing                                                   | ❌     |
| SSH session testing                                               | ❌     |
| BLE connectivity refactor and reliability improvements            | ❌     |

More implementation detail lives in [`docs/ROADMAP.md`](./docs/ROADMAP.md).

## documentation

- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) - technical overview and design notes
- [`docs/TESTED_SETUP.md`](./docs/TESTED_SETUP.md) - verified setup and printer-specific commands
- [`docs/ROADMAP.md`](./docs/ROADMAP.md) - milestones and future work
- [`docs/RELEASING.md`](./docs/RELEASING.md) - packaging and release process

## development

```bash
go build ./...
go test ./...
```

The project is written in Go and uses Bubble Tea/Lip Gloss for the TUI.
