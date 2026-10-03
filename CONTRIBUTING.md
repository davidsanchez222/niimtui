# Contributing to niimtui

Thanks for helping out! `niimtui` is beta software maintained by one person, so small, focused contributions are easiest to review and merge.

## Ways to help

- **Hardware testing.** This is the most valuable contribution right now. Only macOS BLE with the `D110_M` (v4-class) and `B1` is verified. Reports from Linux, Windows, Intel Macs, SSH sessions, or other Niimbot models are very welcome, whether it works or fails.
- **Bug reports.** Open an issue with the details listed [below](#reporting-bugs).
- **Code and docs.** Fixes, features, and README improvements. For anything larger than a small fix, please open an issue first so we can agree on the approach.

## Development setup

You need [Go 1.25+](https://go.dev/dl/). On macOS you also need Xcode Command Line Tools (`xcode-select --install`), because the Bluetooth stack uses cgo.

```bash
git clone https://github.com/davidsanchez222/niimtui.git
cd niimtui
go run ./cmd/niimtui
```

To keep your real config untouched while developing, point niimtui at a throwaway config directory:

```bash
XDG_CONFIG_HOME=/tmp/niimtui-dev go run ./cmd/niimtui
```

A printer is only needed for real prints. Tests and PNG previews (`niimtui preview --out`) run without one.

## Project layout

| path                         | what lives there                               |
| ---------------------------- | ---------------------------------------------- |
| `cmd/niimtui`                | CLI entrypoint, subcommands and setup flow     |
| `internal/tui`               | Bubble Tea designer, gallery, printer sidebar  |
| `internal/label`             | label document model                           |
| `internal/render`            | rendering labels to printer bitmaps and PNGs   |
| `internal/protocol/niimbot`  | Niimbot packet protocol and print tasks        |
| `internal/transport`         | BLE transport and device scanning              |
| `internal/service`           | printing service shared by the CLI, TUI and server |
| `internal/server`, `api`     | local HTTP service (`niimtui serve`)           |
| `internal/config`, `catalog` | config file and built-in label catalog         |

## Before opening a pull request

Run the same checks CI runs:

```bash
go test ./...
go vet ./...
gofmt -l .        # should print nothing
```

Also:

- Add or update tests for behavior changes where practical.
- If you change printing or BLE code, say which printer model and OS you tested on, or that it is untested on hardware.
- For TUI changes, a screenshot or short recording in the PR helps a lot.
- Update the README if user-facing commands, flags, or shortcuts change.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/), with the area as the scope when it helps:

```
feat(tui): add font picker live preview
fix(render): keep QR modules at integer scale
docs: update install instructions
```

Common types: `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `chore`. Common scopes: `tui`, `cli`, `render`, `ble`, `print`, `config`.

## Reporting bugs

Please include:

- the output of `niimtui --version`
- your OS and version, and your Mac's chip (Apple Silicon or Intel) if you're on macOS
- the terminal app you used (this matters for live preview)
- the printer model, and the label size/preset you used
- steps to reproduce, and what you expected compared with what happened
- for slow or failing prints, output from running with `NIIMTUI_PRINT_TIMING=1`

**Redact your config before sharing it.** Remove or replace `auth_token`, `allowed_origins` hostnames, and the printer `device_name`, `identifier` and `address` values.

## Security

Don't open public issues for security problems, such as anything that lets the local service be reached or abused without the auth token. Use [GitHub's private vulnerability reporting](https://github.com/davidsanchez222/niimtui/security/advisories/new) instead.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](./LICENSE).
