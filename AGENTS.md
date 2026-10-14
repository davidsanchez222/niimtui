# AGENTS.md

niimtui designs and prints Niimbot labels from the terminal. The main users are people setting up a Homebox inventory with AI tools, so the CLI is an agent interface as much as a human one.

## Commands

```bash
go test ./...
go vet ./...
gofmt -l .                                   # must print nothing
XDG_CONFIG_HOME="$(mktemp -d)" go run ./cmd/niimtui <subcommand>   # throwaway config
```

These are the CI checks; run all three before calling a change done.

## Layout

The package table is in `CONTRIBUTING.md`. Imports flow one way:

`label` ← `config` ← `render` ← `protocol/niimbot` ← `transport` / `transport/ble` ← `service` ← `server`, `cmd/niimtui`

- `tui` talks to printers through the `PrinterSession` / `PrinterDiscovery` func types that `cmd/niimtui` injects. It never imports `service` or `transport/ble`.
- `cmd/niimtui` and `server` stay thin: they parse input and call `service`, `config` and `render`. Put shared print logic in `service` or `config`, so the CLI, `serve`, `mcp` and the TUI all behave the same.
- What you preview is what prints: the preview and the print go through the same render path in `render`. Keep that true.

## Agent-facing CLI

`print`, `preview`, `designs`, `printers`, `presets`, `scan` and `mcp` are a contract with agents:

- they write JSON to stdout, `{"ok": false, "error": {"code", "message"}}` on failure;
- they exit nonzero on failure;
- they validate every input before they connect to a printer.

When you change their flags or output, update the README `cli` section and the matching MCP tool in `cmd/niimtui/mcp.go`.

## Testing

Tests sit in the same package as the code. Test at these seams:

- `protocol/niimbot`: packet and raster bytes;
- `render`: pixels;
- `service`: session and connection lifecycle, using the fake backend in `service_test.go`;
- `cmd/niimtui`: through `run(args)` and its JSON output;
- `tui`: send messages to `Update` and assert on model state.

Rules:

- Test behavior through those seams, never through private helpers.
- Assert on state, bytes or pixels. Don't assert on view copy, because the copy changes.
- Each test must catch a real regression. A test that restates a constant, embedded data or catalog JSON doesn't.
- Mock only at system boundaries: BLE, the clock, the filesystem.
- Start each bug fix with a failing regression test.

## Conventions

- Conventional commits, with the area as the scope: `feat(tui): …`, `fix(render): …`. The common scopes are listed in `CONTRIBUTING.md`.
- Use the comment density of the code around you, which is sparse. Comment *why*, not *what*.
- Wrap errors with context (`fmt.Errorf("write preview: %w", err)`). In `cmd/niimtui`, a `*commandExit` carries a nonzero exit code that has already been reported as JSON.

## Hardware and BLE

- **Verified hardware:** macOS BLE with the `D110_M` and the `B1`. Everything else is untested.
- **No real hardware:** treat any agent session as having no printer and no Bluetooth permission. Check work with `go test` and `niimtui preview --out <png>`. Read the PNG to check the label.
- **Real devices:** run `print`, `calibrate`, `probe`, `scan`, `serve` or the TUI against a real device only when the user asks. These commands use up label stock and can trigger the macOS Bluetooth permission prompt.
- **Report:** after changing BLE or print code, say which printer and OS you tested on, or that the change is untested on hardware.

## Local-only paths

- `docs/` is gitignored except `docs/RELEASING.md`. It holds the maintainer's notes; put documentation meant for users in `README.md` or `CONTRIBUTING.md`.
- `testlabels/` holds scratch output.
- `~/.config/niimtui/config.json` is the user's real config. During development, point `XDG_CONFIG_HOME` or `--config` at a temp path.
