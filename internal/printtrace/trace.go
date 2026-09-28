// Package printtrace records opt-in timings across a print request.
package printtrace

import (
	"context"
	"log"
	"os"
	"time"
)

type traceKey struct{}

type trace struct {
	start time.Time
}

// Start enables stage timings when NIIMTUI_PRINT_TIMING=1. Repeated calls
// preserve the original start time as the request crosses package boundaries.
func Start(ctx context.Context) context.Context {
	if ctx.Value(traceKey{}) != nil || os.Getenv("NIIMTUI_PRINT_TIMING") != "1" {
		return ctx
	}
	t := &trace{start: time.Now()}
	return context.WithValue(ctx, traceKey{}, t)
}

// Mark logs elapsed time without including label content or printer identifiers.
func Mark(ctx context.Context, stage string) {
	t, ok := ctx.Value(traceKey{}).(*trace)
	if !ok {
		return
	}
	log.Printf("print timing [%d] +%s %s", t.start.UnixNano(), time.Since(t.start).Round(time.Millisecond), stage)
}
