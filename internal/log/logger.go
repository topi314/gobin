package log

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/exp/slog"
	"net/http"
	"time"
)

var StructuredLogger = middleware.RequestLogger(&structuredLogger{})

type structuredLogger struct{}

func (l *structuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	var logFields []slog.Attr
	logFields = append(logFields, slog.String("ts", time.Now().UTC().Format(time.RFC1123)))

	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		logFields = append(logFields, slog.String("req_id", reqID))
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	entry := StructuredLoggerEntry{
		Attrs: append(logFields,
			slog.String("http_scheme", scheme),
			slog.String("http_proto", r.Proto),
			slog.String("http_method", r.Method),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.String("uri", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI))),
		Context: r.Context,
	}

	slog.LogAttrs(r.Context(), slog.LevelInfo, "request started", entry.Attrs...)

	return &entry
}

type StructuredLoggerEntry struct {
	Attrs   []slog.Attr
	Context func() context.Context
}

func (l *StructuredLoggerEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra any) {
	slog.LogAttrs(l.Context(), slog.LevelInfo, "request complete",
		append(l.Attrs, slog.Int("resp_status", status),
			slog.Int("resp_byte_length", bytes),
			slog.Float64("resp_elapsed_ms", float64(elapsed.Nanoseconds())/1000000.0),
		)...,
	)
}

func (l *StructuredLoggerEntry) Panic(v any, stack []byte) {
	slog.LogAttrs(l.Context(), slog.LevelInfo, "",
		append(l.Attrs,
			slog.String("stack", string(stack)),
			slog.String("panic", fmt.Sprintf("%+v", v)),
		)...,
	)
}

func GetLogger(r *http.Request) *slog.Logger {
	entry := middleware.GetLogEntry(r).(*StructuredLoggerEntry)
	return slog.With(entry.Attrs)
}

func AddAttrs(r *http.Request, attrs ...slog.Attr) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Attrs = append(entry.Attrs, attrs...)
	}
}
