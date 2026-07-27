package cli

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(level slog.Level) (*handler, *bytes.Buffer) {
	var buf bytes.Buffer
	h := &handler{
		level: level,
		w:     &buf,
		mu:    &sync.Mutex{},
	}
	return h, &buf
}

var zeroTime = time.Time{}

// ---------------------------------------------------------------------------
// InitLogging verbosity levels
// ---------------------------------------------------------------------------

func TestInitLogging_VerbosityLevels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		level      slog.Level
		logFunc    func(*slog.Logger)
		wantShown  string
		wantHidden string
	}{
		{
			name:       "verbosity 0 (warn) suppresses info, allows warn",
			level:      slog.LevelWarn,
			logFunc:    func(l *slog.Logger) { l.Info("should not appear"); l.Warn("should appear") },
			wantShown:  "should appear",
			wantHidden: "should not appear",
		},
		{
			name:       "verbosity 1 (info) suppresses debug, allows info",
			level:      slog.LevelInfo,
			logFunc:    func(l *slog.Logger) { l.Debug("should not appear"); l.Info("should appear") },
			wantShown:  "should appear",
			wantHidden: "should not appear",
		},
		{
			name:      "verbosity 2 (debug) allows debug and info",
			level:     slog.LevelDebug,
			logFunc:   func(l *slog.Logger) { l.Debug("debug message"); l.Info("info message") },
			wantShown: "debug message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, buf := newTestHandler(tt.level)
			logger := slog.New(h)

			tt.logFunc(logger)

			output := buf.String()
			assert.Contains(t, output, tt.wantShown)
			if tt.wantHidden != "" {
				assert.NotContains(t, output, tt.wantHidden)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Handler level filtering
// ---------------------------------------------------------------------------

func TestHandler_Enabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		handler  slog.Level
		record   slog.Level
		wantEmit bool
	}{
		{"debug handler allows debug", slog.LevelDebug, slog.LevelDebug, true},
		{"debug handler allows info", slog.LevelDebug, slog.LevelInfo, true},
		{"info handler blocks debug", slog.LevelInfo, slog.LevelDebug, false},
		{"info handler allows info", slog.LevelInfo, slog.LevelInfo, true},
		{"info handler allows warn", slog.LevelInfo, slog.LevelWarn, true},
		{"warn handler blocks info", slog.LevelWarn, slog.LevelInfo, false},
		{"warn handler allows warn", slog.LevelWarn, slog.LevelWarn, true},
		{"warn handler allows error", slog.LevelWarn, slog.LevelError, true},
		{"error handler blocks warn", slog.LevelError, slog.LevelWarn, false},
		{"error handler allows error", slog.LevelError, slog.LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(tt.handler)
			got := h.Enabled(context.Background(), tt.record)
			assert.Equal(t, tt.wantEmit, got, "Enabled(%v)", tt.record)
		})
	}
}

// ---------------------------------------------------------------------------
// Prefix formatting
// ---------------------------------------------------------------------------

func TestHandler_Prefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		level      slog.Level
		message    string
		wantPrefix string
	}{
		{"info uses keel prefix", slog.LevelInfo, "test message", "[keel] "},
		{"warn uses warn prefix", slog.LevelWarn, "warning message", "[warn]  "},
		{"error uses error prefix", slog.LevelError, "error message", "[error] "},
		{"debug uses debug prefix", slog.LevelDebug, "debug message", "[debug] "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, buf := newTestHandler(tt.level)
			r := slog.NewRecord(zeroTime, tt.level, tt.message, 0)
			_ = h.Handle(context.Background(), r)

			output := buf.String()
			assert.True(t, len(output) >= len(tt.wantPrefix) && output[:len(tt.wantPrefix)] == tt.wantPrefix,
				"expected prefix %q, got: %q", tt.wantPrefix, output)
		})
	}
}

// ---------------------------------------------------------------------------
// Structured attributes
// ---------------------------------------------------------------------------

func TestHandler_InlineAttributes(t *testing.T) {
	t.Parallel()
	h, buf := newTestHandler(slog.LevelInfo)
	r := slog.NewRecord(zeroTime, slog.LevelInfo, "compiling", 0)
	r.AddAttrs(
		slog.String("agent", "claude"),
		slog.Int("layers", 3),
	)
	_ = h.Handle(context.Background(), r)

	output := buf.String()
	assert.Contains(t, output, "agent=claude")
	assert.Contains(t, output, "layers=3")
	// Verify format: prefix + message + attrs on one line
	assert.True(t, len(output) >= 16 && output[:16] == "[keel] compiling",
		"expected line to start with '[keel] compiling', got: %q", output)
}

func TestHandler_WithAttrs(t *testing.T) {
	t.Parallel()
	h, buf := newTestHandler(slog.LevelInfo)

	// Create a derived handler with pre-computed attrs
	derived := h.WithAttrs([]slog.Attr{
		slog.String("service", "keel"),
	})

	r := slog.NewRecord(zeroTime, slog.LevelInfo, "starting", 0)
	r.AddAttrs(slog.String("agent", "claude"))
	_ = derived.Handle(context.Background(), r)

	output := buf.String()
	assert.Contains(t, output, "service=keel")
	assert.Contains(t, output, "agent=claude")
}

func TestHandler_WithAttrs_Empty(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(slog.LevelInfo)
	derived := h.WithAttrs(nil)
	// Should return the same handler when no attrs are given
	assert.Same(t, h, derived, "WithAttrs(nil) should return the same handler")
}

func TestHandler_WithAttrs_Multiple(t *testing.T) {
	t.Parallel()
	h, buf := newTestHandler(slog.LevelInfo)

	// Chain WithAttrs calls
	h2 := h.WithAttrs([]slog.Attr{slog.String("a", "1")}).(*handler)
	h3 := h2.WithAttrs([]slog.Attr{slog.String("b", "2")})

	r := slog.NewRecord(zeroTime, slog.LevelInfo, "msg", 0)
	_ = h3.Handle(context.Background(), r)

	output := buf.String()
	assert.Contains(t, output, "a=1")
	assert.Contains(t, output, "b=2")
}

// ---------------------------------------------------------------------------
// WithGroup is a documented no-op
// ---------------------------------------------------------------------------

func TestHandler_WithGroup_NoOp(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(slog.LevelInfo)
	derived := h.WithGroup("test-group")
	assert.Same(t, h, derived, "WithGroup should return the same handler (no-op for CLI)")
}

// ---------------------------------------------------------------------------
// Full integration: InitLogging sets up global slog
// ---------------------------------------------------------------------------

func TestInitLogging_SetsGlobalLogger(t *testing.T) {
	// Save and restore default handler
	originalHandler := slog.Default().Handler()
	defer slog.SetDefault(slog.New(originalHandler))

	InitLogging(1) // info level

	// Verify the default handler was replaced
	h := slog.Default().Handler()
	_, ok := h.(*handler)
	require.True(t, ok, "expected handler to be *cli.handler, got %T", h)
}

// ---------------------------------------------------------------------------
// Output includes newline
// ---------------------------------------------------------------------------

func TestHandler_OutputEndsWithNewline(t *testing.T) {
	t.Parallel()
	h, buf := newTestHandler(slog.LevelInfo)
	r := slog.NewRecord(zeroTime, slog.LevelInfo, "test", 0)
	_ = h.Handle(context.Background(), r)

	output := buf.String()
	assert.True(t, len(output) > 0 && output[len(output)-1] == '\n',
		"output should end with newline, got: %q", output)
}
