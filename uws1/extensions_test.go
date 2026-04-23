package uws1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractExtensionsSurfacesMalformedValue(t *testing.T) {
	raw := map[string]json.RawMessage{
		"x-broken": json.RawMessage("[invalid"),
	}
	_, err := extractExtensions(raw, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `extension "x-broken"`)
}

func TestExtractExtensionsSkipsKnownFields(t *testing.T) {
	raw := map[string]json.RawMessage{
		"x-broken": json.RawMessage("[invalid"),
	}
	ext, err := extractExtensions(raw, []string{"x-broken"})
	require.NoError(t, err)
	require.Nil(t, ext)
}

func TestExtractExtensionsPropagatesSyntaxError(t *testing.T) {
	raw := map[string]json.RawMessage{
		"x-ok":     json.RawMessage(`"value"`),
		"x-number": json.RawMessage(`{"a": [}`),
	}
	_, err := extractExtensions(raw, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, `extension "x-number"`)
}
