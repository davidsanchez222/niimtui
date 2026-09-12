# Roadmap

## Direction

The first version of `niimtui` is a local CLI and HTTPS service for Niimbot BLE printing.

Primary workflow:

```text
Homebox page in browser
-> Tampermonkey userscript
-> HTTPS request over Tailscale
-> niimtui serve on MacBook
-> BLE
-> Niimbot printer
```

Version 1 does not target Homebox upstream integration, MQTT, or broad transport support. It targets one stable personal workflow first.

## V1 Scope

Required for version 1:

- macOS
- BLE only
- D110 and B1
- JSON config
- authenticated HTTPS service
- synchronous `POST /print`
- PNG label image input
- direct image print path first
- service-rendered composition later
- named printer profile selection

Explicitly deferred:

- RFCOMM
- USB
- MQTT
- Homebox core integration
- async job queueing
- broad multi-printer ecosystem support

## Contract First

The first implementation milestone is the local contract, not transport breadth.

Finalize these before deep printer work:

- request schema for `POST /print`
- response schema and stable error codes
- JSON config schema
- printer profile resolution rules
- preset resolution rules
- authentication model

Initial error codes should include:

- `UNAUTHORIZED`
- `INVALID_REQUEST`
- `INVALID_IMAGE`
- `PRINTER_NOT_FOUND`
- `PRESET_NOT_FOUND`
- `BLE_CONNECT_FAILED`
- `BLE_WRITE_FAILED`
- `RENDER_FAILED`
- `PRINT_FAILED`

## API Plan

Version 1 endpoints:

- `GET /health`
- `GET /printers`
- `GET /presets`
- `POST /print`

`POST /print` should:

1. authenticate the request
2. validate the JSON body
3. resolve the printer profile
4. resolve the label preset
5. decode and validate the input PNG image
6. render or accept the final label image
7. connect to the printer over BLE
8. print synchronously
9. return structured success or failure

## Config Plan

Use JSON for v1 config.

The config should contain:

- server listen address
- auth token
- optional TLS settings
- named printer profiles
- named label presets

Printer profiles should include:

- `name`
- `model`
- `transport`
- `device_name`
- optional `address`
- `default_preset`
- defaults such as density and rotation

Presets should include:

- `name`
- `width_mm`
- `height_mm`
- `shape`
- `layout`
- `margins_mm`

## Rendering Plan

The service owns final label composition.

Input:

- PNG label image
- selected printer profile
- selected preset
- optional title
- optional subtitle
- optional copy count

Rendering pipeline:

1. decode base64 PNG
2. validate payload size and image dimensions
3. compute preset canvas size
4. place QR inside printable bounds
5. add optional text if layout requires it
6. convert to monochrome
7. rotate or orient for the target model
8. hand off to packet generation

Initial layouts:

- `full-image`
- `qr-only`
- `qr-title`
- `qr-title-subtitle`

The first validated path is `full-image`. QR-only composition modes can follow after the direct image path is stable.

## Printer Model Plan

Model support should be explicit and capability-driven.

Track at least:

- printhead width in pixels
- orientation rules
- density defaults
- print-task variant
- any model-specific packet sequencing differences

Initial support order:

1. `D110_M` v4-class devices
2. `B1`
3. classic `D110`

The first verified path should use whichever printer is easiest to get working reliably over macOS BLE.

## Milestones

### Milestone 1: Service Contract And Scaffolding

- define `POST /print` schema
- define success and error response schema
- define JSON config schema
- stub `GET /health`
- stub `GET /printers`
- stub `GET /presets`
- stub authenticated `POST /print`

Exit criteria:

- service starts from config
- auth works
- invalid requests return stable error codes
- printer and preset resolution paths are wired

### Milestone 2: Rendering Pipeline

- decode base64 PNG input
- validate image payload
- implement preset-aware canvas sizing
- implement `full-image` layout
- implement monochrome conversion
- support model-aware orientation path
- add direct CLI render/print testing path

Exit criteria:

- deterministic rendered label output for at least one preset
- rendering can be exercised without BLE printing

### Milestone 3: macOS BLE Transport

- choose BLE backend
- connect to named printer by configured device name
- implement transport lifecycle
- add timeouts and retry boundaries
- add structured transport logs

Exit criteria:

- one configured printer can be connected to reliably from the MacBook

### Milestone 4: First Working Print Path

- implement packet framing
- implement first printer print-task sequence
- verify one direct-image label prints end-to-end
- return success or failure from `POST /print`

Exit criteria:

- one successful end-to-end BLE print from the HTTPS API

### Milestone 5: Broader Model Expansion

- verify preset differences between D110 and B1
- validate model-specific orientation and label sizing
- add device-type-aware task selection where needed
- support multiple configured printer selectors

Exit criteria:

- both D110_M v4-class devices and B1 work through the same API with different profiles

### Milestone 6: Tampermonkey Integration

- create userscript request flow
- send bearer-authenticated HTTPS request over Tailscale
- handle service responses cleanly in the browser UI
- optionally expose printer or preset selection in the userscript

Exit criteria:

- one-click print from Homebox page through the userscript

## First Vertical Slice

Build this exact slice before broadening scope:

- `niimtui serve`
- JSON config file
- one printer profile: `d110-desk`
- one preset: `d110-12x40`
- one layout: `full-image`
- one endpoint: `POST /print`
- one successful macOS BLE print from PNG label image base64

That slice proves the entire architecture without overcommitting to extra transports, layouts, or printer coverage.

## Validation Rules

Validate at the API boundary:

- bearer token must be present
- printer selector must exist
- preset must exist
- image payload must decode as PNG
- payload must be within size limits
- copies must be within sane bounds
- text fields must be length-limited

Validate before rendering:

- preset dimensions must be finite and positive
- margins must leave printable area
- output dimensions must stay within printer-safe bounds

Validate before printing:

- target model must be supported
- BLE device must be reachable
- print packet generation must succeed

## Security Plan

Version 1 security requirements:

- bearer token authentication on every request
- request body size limits
- PNG-only input for print requests
- metadata-focused logging by default
- bind to Tailscale interface when practical
- configurable TLS support

By default, the service should avoid logging raw base64 QR payloads.

## Testing Plan

Unit tests:

- config parsing and validation
- request validation
- printer profile resolution
- preset resolution
- PNG decode handling
- layout rendering snapshots
- monochrome conversion
- model capability selection
- error code behavior

Integration tests:

- `POST /print` validation path with mocked printing
- render pipeline with fixture PNG label images
- BLE transport behind an interface with mocked backend behavior

Hardware verification:

- D110_M v4 on macOS BLE
- B1 on macOS BLE
- one preset each first
- full-image first, composition layouts later

## Future Expansion

After the first BLE service path is stable, possible next steps include:

- `qr-only`, `qr-title`, and `qr-title-subtitle` composition layouts
- richer preset libraries for B1 label shapes and sizes
- `niimtui print` parity with service mode
- printer discovery command
- async job tracking if synchronous mode becomes limiting
- RFCOMM or USB support if there is a concrete need
- MQTT integration if another producer eventually needs it
