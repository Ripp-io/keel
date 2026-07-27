// Package cli provides CLI-friendly logging via slog.
package cli

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// handler emits clean CLI output: level prefix + message + attributes.
//
// Output format:
//
//	[keel] message text key=value key2=value2
//	[warn]  message text key=value
//	[error] message text key=value
//	[debug] message text key=value
type handler struct {
	level    slog.Leveler
	w        io.Writer
	mu       *sync.Mutex
	preAttrs []slog.Attr
}

// InitLogging configures slog with a CLI-friendly handler.
//
// Verbosity levels:
//
//	0 → warn and above
//	1 → info and above
//	2+ → debug and above
func InitLogging(verbosity int) {
	var level slog.Level
	switch {
	case verbosity >= 2:
		level = slog.LevelDebug
	case verbosity == 1:
		level = slog.LevelInfo
	default:
		level = slog.LevelWarn
	}
	h := &handler{
		level: level,
		w:     os.Stderr,
		mu:    &sync.Mutex{},
	}
	slog.SetDefault(slog.New(h))
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var prefix string
	switch {
	case r.Level >= slog.LevelError:
		prefix = "[error] "
	case r.Level >= slog.LevelWarn:
		prefix = "[warn]  "
	case r.Level >= slog.LevelInfo:
		prefix = "[keel] "
	default:
		prefix = "[debug] "
	}

	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(r.Message)

	// Append pre-computed attributes from WithAttrs.
	for _, a := range h.preAttrs {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(a.Value.String())
	}

	// Append inline attributes from the log call.
	r.Attrs(func(a slog.Attr) bool {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(a.Value.String())
		return true
	})

	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

// WithAttrs returns a new handler that includes the given pre-computed
// attributes in every log line.
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	combined := make([]slog.Attr, 0, len(h.preAttrs)+len(attrs))
	combined = append(combined, h.preAttrs...)
	combined = append(combined, attrs...)
	return &handler{
		level:    h.level,
		w:        h.w,
		mu:       h.mu,
		preAttrs: combined,
	}
}

// WithGroup is a no-op for the CLI handler. Grouped logging adds complexity
// that is unnecessary for CLI output; all attributes are rendered flat.
func (h *handler) WithGroup(_ string) slog.Handler { return h }
