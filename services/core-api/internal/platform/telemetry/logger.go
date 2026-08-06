// Package telemetry provides bounded, redacted operational telemetry for
// ITEMBA-Z runtimes. It never exports request bodies or unrestricted labels.
package telemetry

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var (
	urlCredentialPattern = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s:]+:[^/@\s]+@`)
	bearerPattern        = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	assignmentPattern    = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key)=([^&\s]+)`)
)

// NewJSONLogger returns a structured logger that redacts protected keys and
// credential-shaped values before they reach the configured writer.
func NewJSONLogger(writer io.Writer) *slog.Logger {
	return slog.New(RedactingHandler{next: slog.NewJSONHandler(writer, nil)})
}

// RedactingHandler wraps another slog handler and applies the same policy to
// direct, grouped and pre-bound attributes.
type RedactingHandler struct{ next slog.Handler }

func (handler RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return handler.next.Enabled(ctx, level)
}

func (handler RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, redactText(record.Message), record.PC)
	record.Attrs(func(attribute slog.Attr) bool {
		clean.AddAttrs(redactAttribute(attribute))
		return true
	})
	return handler.next.Handle(ctx, clean)
}

func (handler RedactingHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attributes))
	for _, attribute := range attributes {
		clean = append(clean, redactAttribute(attribute))
	}
	return RedactingHandler{next: handler.next.WithAttrs(clean)}
}

func (handler RedactingHandler) WithGroup(name string) slog.Handler {
	return RedactingHandler{next: handler.next.WithGroup(name)}
}

func redactAttribute(attribute slog.Attr) slog.Attr {
	attribute.Value = attribute.Value.Resolve()
	if sensitiveKey(attribute.Key) {
		return slog.String(attribute.Key, redacted)
	}
	if attribute.Value.Kind() == slog.KindGroup {
		members := attribute.Value.Group()
		clean := make([]slog.Attr, 0, len(members))
		for _, member := range members {
			clean = append(clean, redactAttribute(member))
		}
		return slog.Group(attribute.Key, attrsToAny(clean)...)
	}
	switch attribute.Value.Kind() {
	case slog.KindString:
		return slog.String(attribute.Key, redactText(attribute.Value.String()))
	case slog.KindAny:
		if err, ok := attribute.Value.Any().(error); ok {
			return slog.String(attribute.Key, redactText(err.Error()))
		}
	}
	return attribute
}

func attrsToAny(attributes []slog.Attr) []any {
	result := make([]any, len(attributes))
	for index := range attributes {
		result[index] = attributes[index]
	}
	return result
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, marker := range []string{"authorization", "cookie", "password", "passwd", "secret", "token", "private_key", "request_body", "response_body", "payload"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func redactText(value string) string {
	value = urlCredentialPattern.ReplaceAllString(value, `${1}`+redacted+`@`)
	value = bearerPattern.ReplaceAllString(value, "Bearer "+redacted)
	return assignmentPattern.ReplaceAllString(value, `${1}=`+redacted)
}
