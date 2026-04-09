package identity

import (
	"net/http"
	"strings"

	"github.com/trevorashby/llamasitter/internal/model"
)

const (
	HeaderClientType     = "X-LlamaSitter-Client-Type"
	HeaderClientInstance = "X-LlamaSitter-Client-Instance"
	HeaderAgentName      = "X-LlamaSitter-Agent-Name"
	HeaderSessionID      = "X-LlamaSitter-Session-Id"
	HeaderRunID          = "X-LlamaSitter-Run-Id"
	HeaderWorkspace      = "X-LlamaSitter-Workspace"
)

type Resolved struct {
	model.Identity
	ExtraTags map[string]string
}

func Resolve(headers http.Header, defaults map[string]string) Resolved {
	resolved := Resolved{
		ExtraTags: map[string]string{},
	}

	for key, value := range defaults {
		key = normalizeKey(key)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		switch key {
		case "client_type":
			resolved.ClientType = value
		case "client_instance":
			resolved.ClientInstance = value
		case "agent_name":
			resolved.AgentName = value
		case "session_id":
			resolved.SessionID = value
		case "run_id":
			resolved.RunID = value
		case "workspace":
			resolved.Workspace = value
		default:
			resolved.ExtraTags[key] = value
		}
	}

	overrideFromHeader(&resolved, headers, HeaderClientType, func(value string) { resolved.ClientType = value })
	overrideFromHeader(&resolved, headers, HeaderClientInstance, func(value string) { resolved.ClientInstance = value })
	overrideFromHeader(&resolved, headers, HeaderAgentName, func(value string) { resolved.AgentName = value })
	overrideFromHeader(&resolved, headers, HeaderSessionID, func(value string) { resolved.SessionID = value })
	overrideFromHeader(&resolved, headers, HeaderRunID, func(value string) { resolved.RunID = value })
	overrideFromHeader(&resolved, headers, HeaderWorkspace, func(value string) { resolved.Workspace = value })

	if len(resolved.ExtraTags) == 0 {
		resolved.ExtraTags = nil
	}

	return resolved
}

func overrideFromHeader(resolved *Resolved, headers http.Header, key string, apply func(string)) {
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return
	}
	apply(value)
}

func normalizeKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
