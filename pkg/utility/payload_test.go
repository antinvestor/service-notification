package utility

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPayloadString(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"s": "text", "f": float64(63902), "big": float64(1234567890123), "b": true, "n": nil, "i": 7,
	}
	require.Equal(t, "text", PayloadString(payload, "s"))
	require.Equal(t, "63902", PayloadString(payload, "f"))
	require.Equal(t, "1234567890123", PayloadString(payload, "big"))
	require.Equal(t, "true", PayloadString(payload, "b"))
	require.Equal(t, "7", PayloadString(payload, "i"))
	require.Equal(t, "", PayloadString(payload, "n"))
	require.Equal(t, "", PayloadString(payload, "missing"))
}

func TestPayloadInt(t *testing.T) {
	t.Parallel()

	payload := map[string]any{"f": float64(63902), "s": "42", "bad": "x", "b": true}

	n, ok := PayloadInt(payload, "f")
	require.True(t, ok)
	require.Equal(t, 63902, n)

	n, ok = PayloadInt(payload, "s")
	require.True(t, ok)
	require.Equal(t, 42, n)

	_, ok = PayloadInt(payload, "bad")
	require.False(t, ok)
	_, ok = PayloadInt(payload, "b")
	require.False(t, ok)
	_, ok = PayloadInt(payload, "missing")
	require.False(t, ok)
}

func TestPayloadStrings(t *testing.T) {
	t.Parallel()
	require.Equal(t, map[string]any{"a": "1", "b": "x"}, PayloadStrings(map[string]any{"a": float64(1), "b": "x"}))
}

func TestDeterministicID(t *testing.T) {
	t.Parallel()

	a := DeterministicID("wa", "wamid.HBgLMTIzNDU2Nzg5MBUCABIYFjNFQjBDMUM4RjA1QzRDQjhDQjEyAA==")
	b := DeterministicID("wa", "wamid.HBgLMTIzNDU2Nzg5MBUCABIYFjNFQjBDMUM4RjA1QzRDQjhDQjEyAA==")
	c := DeterministicID("at", "wamid.HBgLMTIzNDU2Nzg5MBUCABIYFjNFQjBDMUM4RjA1QzRDQjhDQjEyAA==")

	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.Regexp(t, `^[0-9a-z_-]{3,40}$`, a)
	require.LessOrEqual(t, len(a), 40)
}
