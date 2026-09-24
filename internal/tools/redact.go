package tools

import (
	"strings"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// Redacted replaces the value of credential-like keys in tool output.
const Redacted = "***REDACTED***"

// sensitiveKeyParts are matched case-insensitively as substrings of map keys.
// Over-redaction (e.g. "AuthMode") is accepted in exchange for never leaking a
// credential stored in EdgeX protocol properties, device properties or
// resource attributes.
var sensitiveKeyParts = []string{
	"password", "passwd", "pwd", "secret", "token", "apikey", "api_key",
	"key", "credential", "auth", "private", "cert", "community",
}

// IsSensitiveKey reports whether a map key likely names a credential.
func IsSensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	for _, p := range sensitiveKeyParts {
		if strings.Contains(lk, p) {
			return true
		}
	}
	return false
}

// RedactMap returns a deep copy of m in which the values of sensitive keys
// are replaced by Redacted, at any nesting depth, and credentials embedded in
// URL-like string values (scheme://user:pass@host) are scrubbed. It returns
// nil for nil input.
func RedactMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if IsSensitiveKey(k) {
			out[k] = Redacted
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

// RedactProtocols redacts every protocol's property map.
func RedactProtocols(p map[string]map[string]any) map[string]any {
	if p == nil {
		return nil
	}
	out := make(map[string]any, len(p))
	for name, props := range p {
		if IsSensitiveKey(name) {
			out[name] = Redacted
			continue
		}
		out[name] = RedactMap(props)
	}
	return out
}

// RedactValue redacts an arbitrary decoded JSON value (maps, slices, strings).
func RedactValue(v any) any { return redactValue(v) }

func redactValue(v any) any {
	switch t := v.(type) {
	case string:
		return edgex.ScrubURLUserinfo(t)
	case map[string]any:
		return RedactMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = redactValue(e)
		}
		return out
	default:
		return v
	}
}
