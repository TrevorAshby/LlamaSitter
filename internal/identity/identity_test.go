package identity

import (
	"net/http"
	"testing"
)

func TestResolveAppliesHeaderPrecedence(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	headers.Set(HeaderClientType, "opencode")
	headers.Set(HeaderSessionID, "session-123")

	resolved := Resolve(headers, map[string]string{
		"client_type":     "default-client",
		"client_instance": "worker-a",
		"workspace":       "/tmp/example",
		"environment":     "local",
	})

	if resolved.ClientType != "opencode" {
		t.Fatalf("expected header override for client type, got %q", resolved.ClientType)
	}
	if resolved.ClientInstance != "worker-a" {
		t.Fatalf("expected default client instance, got %q", resolved.ClientInstance)
	}
	if resolved.SessionID != "session-123" {
		t.Fatalf("expected header session id, got %q", resolved.SessionID)
	}
	if resolved.ExtraTags["environment"] != "local" {
		t.Fatalf("expected extra tag to be preserved")
	}
}
