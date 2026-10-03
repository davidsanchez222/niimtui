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
      alt="Built with Go 1.25"
      src="https://img.shields.io/badge/Go-1.25-8bd5ca?style=for-the-badge&logo=go&logoColor=D9E0EE&labelColor=302D41"
    />
    <img
      alt="OS: macOS"
      src="https://img.shields.io/badge/OS-macOS-8aadf3?style=for-the-badge&logo=apple&logoColor=D9E0EE&labelColor=302D41"
    />
  </p>
  <p>
    <strong>terminal-first label design and printing for Niimbot printers</strong>
  </p>
  <p>
    design labels in a TUI, preview them, and print directly to supported devices.
  </p>
  <p>
    <a href="#install">install</a>
    ·
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
> only keystrokes were used to make the label above! however, niimtui still has full mouse support

---

`niimtui` is a terminal label designer for Niimbot printers. Design a label, preview exactly what will print, and send it over Bluetooth with no vendor app and no cloud.

- **keyboard-first designer** with full mouse support
- **what you preview is what prints**: the preview and the printer share one render path
- **local-first**: prints straight from your machine over BLE
- **scriptable**: saved designs can be printed from the CLI with new values

## install

```bash
brew install davidsanchez222/tap/niimtui
```

macOS on Apple Silicon is tested. Intel Macs are built but untested. Linux and Windows aren't packaged yet; see [support](#support).

## quick start

```bash
niimtui
```

On first run, `niimtui` scans for your printer, picks a label roll, and writes `~/.config/niimtui/config.json`. After that it opens straight into the designer.

```bash
niimtui setup                              # rerun setup / add a printer
niimtui tui --width-mm 50 --height-mm 30   # ad-hoc label size
niimtui --version
```

Inside the designer, press `?` for every shortcut.

## tui

<table>
  <tr>
    <td width="50%" valign="top">
      <strong>label gallery</strong><br>
      starter templates for each printer, plus your saved designs.
      <!-- <img src="assets/screenshots/gallery.png" alt="label gallery"> -->
    </td>
    <td width="50%" valign="top">
      <strong>font picker</strong><br>
      search installed fonts and preview each one live on the canvas.
      <!-- <img src="assets/screenshots/font-picker.png" alt="font picker"> -->
    </td>
  </tr>
  <tr>
    <td valign="top">
      <strong>inverted colors</strong><br>
      flip black and white for bold, high-contrast labels.
      <!-- <img src="assets/screenshots/inverted.png" alt="inverted color mode"> -->
    </td>
    <td valign="top">
      <strong>live image preview</strong><br>
      a real render inside Kitty, Ghostty and WezTerm, with a Preview.app fallback on macOS.
      <!-- <img src="assets/screenshots/live-preview.png" alt="live image preview"> -->
    </td>
  </tr>
  <tr>
    <td valign="top">
      <strong>printer sidebar</strong><br>
      switch printers and rolls; it auto-connects at startup.
      <!-- <img src="assets/screenshots/printer-sidebar.png" alt="printer sidebar"> -->
    </td>
    <td valign="top">
      <strong>reusable designs</strong><br>
      name text and QR fields, save the design, then copy its ready-to-run <code>niimtui print</code> command.
      <!-- <img src="assets/screenshots/saved-designs.png" alt="saved designs"> -->
    </td>
  </tr>
</table>

The designer works best in a terminal of at least `140x30` cells. To control the live preview, set `NIIMTUI_GRAPHICS` to `kitty`, `open` or `off`. WezTerm also needs `enable_kitty_graphics = true`.

## cli

Print a saved design with new values, or a one-off QR label:

```bash
niimtui print --design garage-bin --set url=https://homebox.example/items/123 --set title="Garage Bin 4"
niimtui print --qr https://homebox.example/items/123 --title "Garage Bin 4" --subtitle "Top Shelf"
```

Render a PNG without printing, list saved designs, or scan for printers:

```bash
niimtui preview --design garage-bin --set title="Garage Bin 4" --out ./preview.png
niimtui designs
niimtui scan
```

`--printer`, `--preset` and `--config` override the defaults. Commands print JSON to stdout, and a failed print exits nonzero. Run `niimtui help` to see every command.

## configuration

`niimtui setup` writes `~/.config/niimtui/config.json`, or `$XDG_CONFIG_HOME/niimtui/config.json` if that's set. The TUI keeps it up to date as you add printers and save designs, so you rarely need to edit it by hand.

- `printers`: one profile per device. `default_preset` picks its label roll, and `defaults` holds density and print offsets.
- `presets`: label rolls with size, shape (`rect` or `round`), layout and margins.
- `design_presets`: designs saved from the TUI with `s`. `bindings` map names like `url` and `title` to elements, so `--set url=...` can fill them in from the CLI.
- `server`: settings for `niimtui serve`. Keep `auth_token` secret.

<details>
<summary>example config</summary>

```json
{
  "server": {
    "listen": "127.0.0.1:8443",
    "auth_token": "replace-with-a-long-random-token"
  },
  "active_printer": "b1-desk",
  "printers": [
    {
      "name": "b1-desk",
      "model": "B1",
      "transport": "ble",
      "device_name": "B1-XXXXXXXXXX",
      "identifier": "00000000-0000-0000-0000-000000000000",
      "default_preset": "b1-50x30",
      "defaults": { "density": 3, "offset_x_mm": 0, "offset_y_mm": 0 }
    },
    {
      "name": "d110-shelf",
      "model": "D110",
      "transport": "ble",
      "device_name": "D110_M-XXXXXXXXXX",
      "default_preset": "d110-12x40",
      "defaults": { "density": 3 }
    }
  ],
  "presets": [
    {
      "name": "b1-50x30",
      "width_mm": 50,
      "height_mm": 30,
      "shape": "rect",
      "layout": "qr-title",
      "margins_mm": 2
    },
    {
      "name": "d110-12x40",
      "width_mm": 40,
      "height_mm": 12,
      "shape": "rect",
      "layout": "qr-only",
      "margins_mm": 1
    }
  ],
  "design_presets": [
    {
      "name": "garage-bin",
      "document": {
        "WidthMM": 50,
        "HeightMM": 30,
        "Shape": "rect",
        "Elements": [
          {
            "ID": "qr",
            "Type": "qr",
            "XMM": 2,
            "YMM": 2,
            "WidthMM": 26,
            "HeightMM": 26,
            "QR": { "Value": "https://homebox.example/items/123" }
          },
          {
            "ID": "title",
            "Type": "text",
            "XMM": 30,
            "YMM": 8,
            "WidthMM": 18,
            "HeightMM": 14,
            "Text": {
              "Value": "Garage Bin 4",
              "FontSize": 40,
              "Bold": true,
              "Align": "left"
            }
          }
        ]
      },
      "bindings": [
        { "name": "url", "element_id": "qr", "required": true },
        { "name": "title", "element_id": "title" }
      ]
    }
  ]
}
```

</details>

## support

`niimtui` is beta software.

| OS          | Transport  | Status |
| ----------- | ---------- | ------ |
| macOS       | BLE        | ✅     |
| macOS       | Serial/USB | ❓     |
| Linux       | BLE        | ❓     |
| Windows     | BLE        | ❓     |
| SSH session | TUI        | ❓     |

✅ verified · ❓ untested. Verified printers: `D110_M` (v4-class) and `B1`.

## homebox and service mode

The goal is a local `niimtui` service that accepts authenticated print requests from Homebox-style workflows, with all rendering and printer handling kept on your machine. `niimtui serve` already handles QR labels with an optional title and subtitle.

## roadmap

| Item                                                             | Status |
| ---------------------------------------------------------------- | ------ |
| Support for ALL niimbot printers                                 | ❌     |
| Serial/USB support                                               | ❌     |
| Full HTTPS Homebox integration                                   | ❌     |
| Automatic RFID label roll detection                              | ❌     |
| Homebrew Core submission (`brew install niimtui`)                | ❌     |
| Windows package manager publishing: winget, then Scoop           | ❌     |
| Linux packaging: Arch (`niimtui-bin`), then apt `.deb`, then Nix | ❌     |
| Linux testing                                                    | ❌     |
| Windows testing                                                  | ❌     |
| SSH session testing                                              | ❌     |

## development

### prerequisites

- [Go 1.25+](https://go.dev/dl/) (`brew install go`)
- macOS: Xcode Command Line Tools (`xcode-select --install`). The Bluetooth stack uses cgo, so you need a C compiler.
- A Niimbot printer is only needed for real prints. Tests and PNG previews run without one.

### get started

```bash
git clone https://github.com/davidsanchez222/niimtui.git
cd niimtui
go mod download           # optional; go build/run fetch modules automatically
go run ./cmd/niimtui      # run from source
```

The first time you connect from a terminal, macOS asks for Bluetooth permission for that terminal app. Allow it, or scanning will find nothing.

### build and test

```bash
go build -o niimtui ./cmd/niimtui   # local binary
go test ./...
go vet ./...
gofmt -l .                          # should print nothing
```

CI runs `go test`, `go vet` and `goreleaser check` on macOS for every PR.

### useful environment variables

| variable                 | effect                                                                     |
| ------------------------ | -------------------------------------------------------------------------- |
| `XDG_CONFIG_HOME`        | config location; e.g. `XDG_CONFIG_HOME=/tmp/niim` gives a throwaway config |
| `NIIMTUI_GRAPHICS`       | live preview mode: `kitty`, `open` or `off`                                |
| `NIIMTUI_PRINT_TIMING=1` | log per-stage print timings                                                |
| `NIIMTUI_DEBUG_PANIC=1`  | let TUI panics crash with a full stack trace                               |

Built with Go and Bubble Tea/Lip Gloss. See [`docs/RELEASING.md`](./docs/RELEASING.md) for the release process.
