package websrv

import (
	"context"
	"io"
	"log/slog"
)

// simpleHandler is a slog.Handler that writes human-readable single-line logs
// without the verbose "time=... level=... msg=..." prefix emitted by the
// default TextHandler.
//
// Output format:
//
//	INFO 🚀 Starting server addr=127.0.0.8:80
//	ERROR ❌ Error starting server err=...
type simpleHandler struct {
	w io.Writer
}

// newSimpleHandler returns a simpleHandler that writes to w.
func newSimpleHandler(w io.Writer) *simpleHandler {
	return &simpleHandler{w: w}
}

func (h *simpleHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *simpleHandler) Handle(_ context.Context, r slog.Record) error {
	var b []byte
	b = append(b, r.Level.String()...)
	b = append(b, ' ')
	b = append(b, r.Message...)
	r.Attrs(func(a slog.Attr) bool {
		b = append(b, ' ')
		b = append(b, a.Key...)
		b = append(b, '=')
		b = append(b, a.Value.String()...)
		return true
	})
	b = append(b, '\n')
	_, err := h.w.Write(b)
	return err
}

func (h *simpleHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *simpleHandler) WithGroup(_ string) slog.Handler      { return h }
