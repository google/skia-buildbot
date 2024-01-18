package run_benchmark

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCmd(t *testing.T) {
	for i, test := range []struct {
		name        string
		req         RunBenchmarkRequest
		expectedErr string
	}{
		{
			name: "create command works",
			req: RunBenchmarkRequest{
				Benchmark: "benchmark",
				Story:     "story",
				Commit:    "64893ca6294946163615dcf23b614afe0419bfa3",
			},
			expectedErr: "",
		},
		{
			name: "benchmark error",
			req: RunBenchmarkRequest{
				Story:  "story",
				Commit: "64893ca6294946163615dcf23b614afe0419bfa3",
			},
			expectedErr: "Benchmark",
		},
		{
			name: "story error",
			req: RunBenchmarkRequest{
				Benchmark: "benchmark",
				Commit:    "64893ca6294946163615dcf23b614afe0419bfa3",
			},
			expectedErr: "Story",
		},
		{
			name: "commit error",
			req: RunBenchmarkRequest{
				Benchmark: "benchmark",
				Story:     "story",
			},
			expectedErr: "Commit",
		},
		{
			name: "base_perftests not yet implemented",
			req: RunBenchmarkRequest{
				Benchmark: "base_perftests",
				Story:     "story",
				Commit:    "64893ca6294946163615dcf23b614afe0419bfa3",
			},
			expectedErr: "base_perftests is not yet implemented",
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			e := telemetryExp{}
			cmd, err := e.createCmd(test.req)
			if test.expectedErr == "" {
				assert.Contains(t, cmd, test.req.Benchmark)
				assert.Contains(t, cmd, fmt.Sprintf("^%s$", test.req.Story))
			} else {
				assert.ErrorContains(t, err, test.expectedErr)
			}
		})
	}
}
