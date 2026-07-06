package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserAttributeValueFromRequestAcceptsStringsAndNumbers(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "string", raw: `"active"`, want: "active"},
		{name: "integer", raw: `25`, want: "25"},
		{name: "decimal", raw: `12.5`, want: "12.5"},
		{name: "null", raw: `null`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := userAttributeValueFromRequest(json.RawMessage(tt.raw))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestUserAttributeValueFromRequestRejectsUnsupportedValues(t *testing.T) {
	for _, raw := range []string{`true`, `{"value":"active"}`, `["active"]`} {
		t.Run(raw, func(t *testing.T) {
			_, err := userAttributeValueFromRequest(json.RawMessage(raw))
			require.Error(t, err)
		})
	}
}
