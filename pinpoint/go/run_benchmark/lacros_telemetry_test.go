package run_benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommand_TargetLacrosX86Perf_ReturnsTestSuiteOctopus(t *testing.T) {
	commit := "01bfa421eee3c76bbbf32510343e074060051c9f"

	b, err := NewBenchmarkTest(commit, "lacros-x86-perf", "", "jetstream2", "", "")
	assert.NoError(t, err)

	cmd := b.GetCommand()
	assert.Contains(t, cmd, "bin/run_performance_test_suite_octopus")
	assert.NotContains(t, cmd, "bin/run_performance_test_suite_eve")
	assert.Contains(t, cmd, "--isolated-script-test-output")
}

func TestGetCommand_TargetLacrosEvePerf_ReturnsTestSuiteEve(t *testing.T) {
	commit := "01bfa421eee3c76bbbf32510343e074060051c9f"

	b, err := NewBenchmarkTest(commit, "lacros-eve-perf", "", "jetstream2", "", "")
	assert.NoError(t, err)

	cmd := b.GetCommand()
	assert.Contains(t, cmd, "bin/run_performance_test_suite_eve")
	assert.NotContains(t, cmd, "bin/run_performance_test_suite_octopus")
	assert.Contains(t, cmd, "--isolated-script-test-output")
}
