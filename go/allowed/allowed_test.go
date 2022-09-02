package allowed

import (
	"testing"
)

func TestAllowed(t *testing.T) {
	testCases := []struct {
		allowed  []string
		value    string
		expected bool
		message  string
	}{
		{
			allowed:  []string{},
			value:    "test@example.org",
			expected: false,
			message:  "empty",
		},
		{
			allowed:  []string{""},
			value:    "test@",
			expected: false,
			message:  "empty domain",
		},
		{
			allowed:  []string{"test@example.org"},
			value:    "test@example.org",
			expected: true,
			message:  "single email",
		},
		{
			allowed:  []string{"example.org"},
			value:    "test@example.org",
			expected: true,
			message:  "single domain",
		},
		{
			allowed:  []string{"example.org"},
			value:    "test@google.com",
			expected: false,
			message:  "single domain fail",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "test@google.com",
			expected: true,
			message:  "multi domain",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "foo@chromium.org",
			expected: true,
			message:  "multi domain 2",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "special@example.com",
			expected: true,
			message:  "multi domain 3",
		},
		{
			allowed:  []string{"google.com", "chromium.org", "special@example.com"},
			value:    "missing@example.com",
			expected: false,
			message:  "multi domain 4",
		},
	}

	for _, tc := range testCases {
		w := NewAllowedFromList(tc.allowed)
		if got, want := w.Member(tc.value), tc.expected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
