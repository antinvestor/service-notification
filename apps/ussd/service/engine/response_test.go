package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponseBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		beginTpl    string
		continueTpl string
		endTpl      string
		message     string
		isBeginning bool
		isEnd       bool
		wantMessage string
		wantIsEnd   bool
	}{
		{
			name:        "beginning with template",
			beginTpl:    "BEGIN %s END",
			continueTpl: "CON %s",
			endTpl:      "BYE %s",
			message:     "Hello",
			isBeginning: true,
			wantMessage: "BEGIN Hello END",
		},
		{
			name:        "continue with template",
			beginTpl:    "BEGIN %s",
			continueTpl: "CON %s",
			endTpl:      "BYE %s",
			message:     "Choose",
			wantMessage: "CON Choose",
		},
		{
			name:        "end with template",
			beginTpl:    "BEGIN %s",
			continueTpl: "CON %s",
			endTpl:      "BYE %s",
			message:     "Goodbye",
			isEnd:       true,
			wantMessage: "BYE Goodbye",
			wantIsEnd:   true,
		},
		{
			name:        "no template",
			message:     "Plain message",
			wantMessage: "Plain message",
		},
		{
			name:        "empty template no substitution",
			beginTpl:    "No placeholder here",
			message:     "Hello",
			isBeginning: true,
			wantMessage: "Hello", // No %s so message stays as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewResponseBuilder(tt.beginTpl, tt.continueTpl, tt.endTpl)
			resp := rb.Build(tt.message, tt.isBeginning, tt.isEnd)
			require.Equal(t, tt.wantMessage, resp.Message)
			require.Equal(t, tt.wantIsEnd, resp.IsEnd)
		})
	}
}

func TestSubstituteSessionData(t *testing.T) {
	data := map[string]any{
		"name":    "John",
		"balance": 1000,
		"account": "ACC-001",
	}

	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "curly brace style",
			message: "Hello {name}, balance is {balance}",
			want:    "Hello John, balance is 1000",
		},
		{
			name:    "python style",
			message: "Hello %(name)s, account %(account)s",
			want:    "Hello John, account ACC-001",
		},
		{
			name:    "no placeholders",
			message: "Plain text",
			want:    "Plain text",
		},
		{
			name:    "missing key",
			message: "Hello {unknown}",
			want:    "Hello {unknown}",
		},
		{
			name:    "mixed styles",
			message: "{name} has %(balance)s in {account}",
			want:    "John has 1000 in ACC-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteSessionData(tt.message, data)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestSubstituteSessionData_NilData(t *testing.T) {
	result := SubstituteSessionData("Hello {name}", nil)
	require.Equal(t, "Hello {name}", result)
}
