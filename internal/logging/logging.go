package logging

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"time"
)

var sensitiveValue = regexp.MustCompile(`(?i)((?:token|secret|password|authorization|api[_-]?key)\s*[=:]\s*)[^\s,;]+`)

type levelFilteringHandler struct {
	next slog.Handler
}

func (h levelFilteringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h levelFilteringHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, bounded(record.Message, 240), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(sanitize(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h levelFilteringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, sanitize(attr))
	}
	return levelFilteringHandler{next: h.next.WithAttrs(clean)}
}

func (h levelFilteringHandler) WithGroup(name string) slog.Handler {
	return levelFilteringHandler{next: h.next.WithGroup(bounded(name, 80))}
}

func New(output io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(levelFilteringHandler{next: handler})
}

func sanitize(attr slog.Attr) slog.Attr {
	switch attr.Key {
	case "duration":
		if duration, ok := attr.Value.Any().(time.Duration); ok {
			return slog.Int64("duration_ms", duration.Milliseconds())
		}
	case "url", "source_url":
		return slog.String(attr.Key, SanitizeURL(attr.Value.String()))
	case "error":
		message := attr.Value.String()
		if err, ok := attr.Value.Any().(error); ok {
			message = err.Error()
		}
		return slog.String("error", bounded(RedactSecrets(message), 320))
	default:
		if attr.Value.Kind() == slog.KindString {
			return slog.String(attr.Key, bounded(RedactSecrets(attr.Value.String()), 320))
		}
	}
	return attr
}

func SanitizeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[invalid-url]"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func RedactSecrets(value string) string {
	return sensitiveValue.ReplaceAllString(value, `${1}[REDACTED]`)
}

func bounded(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
