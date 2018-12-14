package main

import (
	"testing"

	"go.skia.org/infra/go/testutils"
)

func TestEquationFromExpr(t *testing.T) {
	testutils.SmallTest(t)

	testCases := []struct {
		value    string
		expected string
		message  string
	}{
		{
			value:    "up == 0",
			expected: "up",
			message:  "==",
		},
		{
			value:    "liveness_ci_pubsub_receive_s > 60 * 60 * 24 * 2",
			expected: "liveness_ci_pubsub_receive_s",
			message:  ">",
		},
		{
			value:    "cq_watcher_in_flight_waiting_in_cq{app=\"cq-watcher\"} >= 10",
			expected: "cq_watcher_in_flight_waiting_in_cq{app=\"cq-watcher\"}",
			message:  "{app=...}",
		},
		{
			value:    "healthy{app=\"ct-master\"} != 1",
			expected: "healthy{app=\"ct-master\"}",
			message:  "!",
		},
		{
			value:    "",
			expected: "",
			message:  "empty string",
		},
	}

	for _, tc := range testCases {
		if got, want := equationFromExpr(tc.value), tc.expected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
