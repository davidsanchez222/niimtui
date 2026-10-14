//go:build js && wasm

// Command web runs the niimtui designer in a browser tab. The page owns the
// terminal emulator and talks to this program through globalThis.niimtui:
//
//	page sets:    cols, rows, onOutput(Uint8Array), onExit(error?)
//	program sets: input(string), resize(cols, rows)
//
// Printing, live image previews and saving to disk are unavailable here.
package main

import (
	"context"
	"io"
	"syscall/js"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"niimtui/internal/config"
	"niimtui/internal/tui"
)

// input buffers keystrokes from JS callbacks, which must never block.
type input chan []byte

func (in input) Read(p []byte) (int, error) {
	b, ok := <-in
	if !ok {
		return 0, io.EOF
	}
	return copy(p, b), nil
}

type output struct{ onOutput js.Value }

func (o output) Write(p []byte) (int, error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(buf, p)
	o.onOutput.Invoke(buf)
	return len(p), nil
}

func main() {
	host := js.Global().Get("niimtui")

	// Output is not a TTY, so color detection would fall back to plain text.
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)

	keys := make(input, 256)
	model := tui.NewModelWithPresets(50, 30, "rect", "", tui.PrintConfig{
		DiscoveryContext: context.Background(),
		Preferences:      config.Preferences{LivePreview: config.LivePreviewOff},
	}, nil, "")
	p := tea.NewProgram(model,
		tea.WithInput(keys),
		tea.WithOutput(output{host.Get("onOutput")}),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		tea.WithoutSignalHandler(),
	)

	host.Set("input", js.FuncOf(func(_ js.Value, args []js.Value) any {
		select {
		case keys <- []byte(args[0].String()):
		default: // drop input rather than stall the page
		}
		return nil
	}))
	resize := func(cols, rows int) {
		go p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	}
	host.Set("resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		resize(args[0].Int(), args[1].Int())
		return nil
	}))
	resize(host.Get("cols").Int(), host.Get("rows").Int())

	_, err := p.Run()
	close(keys)
	if err != nil {
		host.Call("onExit", err.Error())
		return
	}
	host.Call("onExit")
}
