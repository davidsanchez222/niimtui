# Builds the browser demo into dist/: niimtui.wasm plus Go's JS loader.
GOROOT := $(shell go env GOROOT)

.PHONY: build clean
build:
	mkdir -p dist
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o dist/niimtui.wasm .
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" dist/

clean:
	rm -rf dist
