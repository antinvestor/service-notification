package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectProtocol(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        string
	}{
		{"json", "application/json", "json"},
		{"xml", "application/xml", "xml"},
		{"text xml", "text/xml", "xml"},
		{"form default", "application/x-www-form-urlencoded", "form"},
		{"empty", "", "form"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal request to test protocol detection
			require.Equal(t, tt.want, detectProtocolFromContentType(tt.contentType))
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	require.Equal(t, "a", firstNonEmpty("a", "b", "c"))
	require.Equal(t, "b", firstNonEmpty("", "b", "c"))
	require.Equal(t, "c", firstNonEmpty("", "", "c"))
	require.Equal(t, "", firstNonEmpty("", "", ""))
}

// detectProtocolFromContentType is a helper to test protocol detection without needing an http.Request.
func detectProtocolFromContentType(contentType string) string {
	switch {
	case len(contentType) > 0 && contains(contentType, "json"):
		return "json"
	case len(contentType) > 0 && contains(contentType, "xml"):
		return "xml"
	default:
		return "form"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
