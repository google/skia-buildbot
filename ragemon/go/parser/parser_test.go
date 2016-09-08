package parser

import (
	"reflect"
	"testing"

	"go.skia.org/infra/ragemon/go/store"
)

func TestParser(t *testing.T) {
	testCases := []struct {
		value    string
		expected []store.Measurement
		fail     bool
		message  string
	}{
		{
			value:    "",
			expected: nil,
			fail:     true,
			message:  "Empty should be fine?",
		},
	}

	for _, tc := range testCases {
		parsed, err := PlainText(tc.value)
		if err != nil {
			if !tc.fail {
				t.Errorf("Failed unexpectedly %q: %s %s", tc.value, err, tc.message)
			}
			continue
		}
		if got, want := parsed, tc.expected; !reflect.DeepEqual(got, want) {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
