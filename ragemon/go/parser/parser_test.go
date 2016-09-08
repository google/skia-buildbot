package parser

import (
	"testing"

	"go.skia.org/infra/ragemon/go/store"
	"go.skia.org/infra/ragemon/go/ts"
)

func equalExceptTime(m1, m2 []store.Measurement) bool {
	if len(m1) != len(m2) {
		return false
	}
	for i, m := range m1 {
		if m.Key != m2[i].Key {
			return false
		}
		if m.Point.Value != m2[i].Point.Value {
			return false
		}
	}
	return true
}

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
			message:  "Empty should be error",
		},
		{
			value:    ",foo=bar,",
			expected: nil,
			fail:     true,
			message:  "Malformed line",
		},
		{
			value: ",config=565, 102",
			expected: []store.Measurement{
				store.Measurement{
					Key: ",config=565,",
					Point: ts.Point{
						Value: 102,
					},
				},
			},
			fail:    false,
			message: "One line OK",
		},
		{
			value: ",config=565, 102\n,config=8888, 203",
			expected: []store.Measurement{
				store.Measurement{
					Key: ",config=565,",
					Point: ts.Point{
						Value: 102,
					},
				},
				store.Measurement{
					Key: ",config=8888,",
					Point: ts.Point{
						Value: 203,
					},
				},
			},
			fail:    false,
			message: "Two line OK",
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
		if got, want := parsed, tc.expected; !equalExceptTime(got, want) {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
