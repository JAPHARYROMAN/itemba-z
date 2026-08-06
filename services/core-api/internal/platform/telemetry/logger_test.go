package telemetry

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactingHandlerRemovesProtectedKeysAndCredentialShapes(t *testing.T) {
	var output bytes.Buffer
	logger := NewJSONLogger(&output).With("service", "api", "session_token", "do-not-log")
	logger.Error("request failed with Bearer abc.def", "authorization", "Bearer unsafe", "error", errors.New("connect postgres://user:password@db/itemba?password=unsafe"),
		slog.Group("request", "correlation_id", "safe-id", "cookie", "unsafe-cookie"))
	logged := output.String()
	for _, secret := range []string{"do-not-log", "Bearer abc.def", "Bearer unsafe", "user:password", "password=unsafe", "unsafe-cookie"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log leaked %q: %s", secret, logged)
		}
	}
	for _, retained := range []string{"service", "api", "correlation_id", "safe-id", redacted} {
		if !strings.Contains(logged, retained) {
			t.Fatalf("log omitted %q: %s", retained, logged)
		}
	}
}
