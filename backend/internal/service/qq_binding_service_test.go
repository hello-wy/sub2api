package service

import "testing"

func TestNormalizeQQNumber(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{name: "trims spaces", input: " 12345 ", want: "12345", valid: true},
		{name: "five digits", input: "12345", want: "12345", valid: true},
		{name: "twelve digits", input: "123456789012", want: "123456789012", valid: true},
		{name: "leading zero", input: "012345", valid: false},
		{name: "too short", input: "1234", valid: false},
		{name: "too long", input: "1234567890123", valid: false},
		{name: "not numeric", input: "1234a", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeQQNumber(test.input)
			if test.valid {
				if err != nil || got != test.want {
					t.Fatalf("normalizeQQNumber(%q) = (%q, %v), want (%q, nil)", test.input, got, err, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("normalizeQQNumber(%q) unexpectedly succeeded with %q", test.input, got)
			}
		})
	}
}
