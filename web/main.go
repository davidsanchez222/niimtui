//go:build js && wasm

// Command web runs the niimtui designer in a browser tab. The page owns the
// terminal emulator and talks to this program through globalThis.niimtui:
//
//	page sets:    cols, rows, onOutput(Uint8Array), onExit(error?)
//	program sets: input(string), resize(cols, rows),
//	              mouse(action, button, x, y, alt, ctrl, shift)
//
// Printing, live image previews and saving to disk are unavailable here; the
// printer is a display-only B1 profile.
package main

import (
	"context"
	"errors"
	"io"
	"syscall/js"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/davidsanchez222/niimtui/internal/config"
	"github.com/davidsanchez222/niimtui/internal/tui"
)

var demoPrinter = config.PrinterProfile{
	Name:          "b1",
	Model:         "B1",
	Transport:     "ble",
	DeviceName:    "B1-demo",
	DefaultPreset: "b1-50x30",
}

var demoPresets = []config.LabelPreset{
	{Name: "b1-50x30", WidthMM: 50, HeightMM: 30, Shape: "rect", Layout: "qr-title", MarginsMM: 2},
	{Name: "b1-50x50-round", WidthMM: 50, HeightMM: 50, Shape: "round", Layout: "qr-title-subtitle", MarginsMM: 2},
}

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
		// A display-only printer so the sidebar shows the B1 art, roll and
		// margins; connecting reports that printing is unavailable here.
		NewSession: func(string) (tui.PrinterSession, error) {
			return nil, errors.New("printing is unavailable in the browser demo")
		},
		Printers:         []config.PrinterProfile{demoPrinter},
		Printer:          demoPrinter.Name,
		PreferredPrinter: demoPrinter.Name,
		Model:            demoPrinter.Model,
		DeviceName:       demoPrinter.DeviceName,
		Copies:           1,
	}, demoPresets, demoPrinter.DefaultPreset)
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

	// The terminal emulator doesn't encode mouse reports, so the page sends
	// cell coordinates and Bubble Tea's action and button values directly.
	// One goroutine forwards them so press, drag and release stay in order.
	mouse := make(chan tea.MouseMsg, 256)
	go func() {
		for msg := range mouse {
			p.Send(msg)
		}
	}()
	host.Set("mouse", js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := tea.MouseMsg{
			Action: tea.MouseAction(args[0].Int()),
			Button: tea.MouseButton(args[1].Int()),
			X:      args[2].Int(),
			Y:      args[3].Int(),
			Alt:    args[4].Bool(),
			Ctrl:   args[5].Bool(),
			Shift:  args[6].Bool(),
		}
		select {
		case mouse <- msg:
		default: // drop motion rather than stall the page
		}
		return nil
	}))

	_, err := p.Run()
	close(keys)
	if err != nil {
		host.Call("onExit", err.Error())
		return
	}
	host.Call("onExit")
}
