package log_parser

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/external/rules_python"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/td"
)

func TestRun(t *testing.T) {

	// Create a dummy output format which just indicates results of steps.
	steps := []struct {
		Name   string
		Result td.StepResult
	}{
		{
			Name:   "a",
			Result: td.StepResultSuccess,
		},
		{
			Name:   "b",
			Result: td.StepResultFailure,
		},
		{
			Name:   "c",
			Result: td.StepResultSuccess,
		},
	}
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Run(ctx, ".", []string{"echo", "a b c"}, bufio.ScanWords, func(sm *StepManager, line string) error {
			for _, stepData := range steps {
				if stepData.Name == line {
					s := sm.StartStep(td.Props(stepData.Name))
					if stepData.Result != td.StepResultSuccess {
						s.Fail()
					}
					s.End()
					return nil
				}
			}
			return fmt.Errorf("No matching step for %q", line)
		})
	})

	// Verify that we got the expected structure.
	require.Equal(t, 1, len(res.Steps))
	cmdStep := res.Steps[0]
	require.Equal(t, "echo a b c", cmdStep.Name)
	require.Equal(t, len(steps), len(cmdStep.Steps))
	for idx, actualStep := range cmdStep.Steps {
		require.Equal(t, steps[idx].Name, actualStep.Name)
		require.Equal(t, steps[idx].Result, actualStep.Result)
	}
}

// everyLineIsAStep is a function which may be passed to Run. It emits a new
// step for each line in the stdout stream.
func everyLineIsAStep(sm *StepManager, line string) error {
	s := sm.CurrentStep()
	if s != nil {
		s.End()
	}
	sm.StartStep(td.Props(line))
	return nil
}

func TestTimeout(t *testing.T) {
	// TODO(borenet): This test will not work on Windows.

	// Write a script to generate steps.
	tmp := t.TempDir()
	slowSecondStep := filepath.Join(tmp, "script.sh")
	testutils.WriteFile(t, slowSecondStep, `#!/bin/bash
echo "Step1"
echo "Step2"
sleep 10
echo "Step3"
`)

	// The following Task Driver runs the above script, which takes longer
	// than 10 seconds, with a timeout of 100 milliseconds.
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Set up the timeout.
		timeout := 100 * time.Millisecond
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		start := time.Now()
		defer func() {
			elapsed := time.Now().Sub(start)
			require.True(t, elapsed < 2*time.Second, fmt.Sprintf("Timeout is %s; command exited after %s", timeout, elapsed))
		}()

		// Run the script. It should hit the timeout during the second
		// step.
		return Run(ctx, ".", []string{slowSecondStep}, bufio.ScanLines, everyLineIsAStep)
	})

	// There should be a single root-level step, which is the execution of
	// the script itself.
	require.Equal(t, 1, len(res.Steps))
	require.Equal(t, td.StepResultFailure, res.Steps[0].Result)

	// We saw two log lines before the timeout, so we should have two steps.
	// The second should be a failure because of the timeout.
	require.Equal(t, 2, len(res.Steps[0].Steps))
	require.Equal(t, td.StepResultSuccess, res.Steps[0].Steps[0].Result)
	require.Equal(t, td.StepResultFailure, res.Steps[0].Steps[1].Result)

	// The active steps should have received errors in their logs.
	assertLogMatchesContent(t, res.Steps[0], logNameStderr, context.DeadlineExceeded.Error()+"\n")
	assertLogMatchesContent(t, res.Steps[0].Steps[1], logNameStderr, context.DeadlineExceeded.Error()+"\n")
}

// numberedStepsRe matches lines like "Step 1: Hello World" to produce steps.
var numberedStepsRe = regexp.MustCompile(`^Step \d: (.+)$`)

// numberedStepsTokenHandler is a TokenHandler which uses numberedStepsRe.
var numberedStepsTokenHandler = RegexpTokenHandler(numberedStepsRe)

// runPythonScript writes the given Python script to a temporary file and runs
// a Task Driver which uses log_parser.Run with the given TokenHandler.
func runPythonScript(t *testing.T, fn TokenHandler, script string) *td.StepReport {
	python3, err := rules_python.FindPython3()
	require.NoError(t, err)

	// Write a script to generate steps.
	tmp := t.TempDir()
	scriptPath := filepath.Join(tmp, "script.py")
	testutils.WriteFile(t, scriptPath, script)

	// Run the Task Driver.
	return td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Run(ctx, ".", []string{python3, "-u", scriptPath}, bufio.ScanLines, fn)
	})
}

// assertLogMatchesContent verifies that the given log buffer contains the given
// lines.
func assertLogMatchesContent(t *testing.T, s *td.StepReport, logName, expect string) {
	var b *bytes.Buffer
	for _, data := range s.Data {
		logData, ok := data.(*td.LogData)
		if ok && logData.Name == logName {
			log, ok := s.Logs[logData.Id]
			if ok {
				b = log
				break
			}
		}
	}
	require.NotNil(t, b, "Failed to find log %q for step %q", logName, s.Name)
	require.Equal(t, expect, b.String())
}

func TestLogs(t *testing.T) {
	unittest.BazelOnlyTest(t) // Uses the Bazel-downloaded python3 binary.

	// This script writes log output which implies two sub-steps. We expect
	// the numberedStepsTokenHandler to actually emit these two steps, and
	// we expect that all log output which follows "Step X" lines to be
	// attributed to step X.
	res := runPythonScript(t, numberedStepsTokenHandler, `
from __future__ import print_function
import sys
import time

print('Step 1: Do a thing')
print('log for step 1')
print('... more')
print('Step 2: Do another thing')
print('inside step 2')
print('err in step 2', file=sys.stderr)
print('more err in step 2', file=sys.stderr)
`)

	// Run should produce one root-level step, which is the execution of the
	// command itself. This step should have all of the raw output of the
	// command.
	require.Equal(t, 1, len(res.Steps))
	base := res.Steps[0]
	require.Equal(t, td.StepResultSuccess, base.Result)
	assertLogMatchesContent(t, base, logNameStdout, `Step 1: Do a thing
log for step 1
... more
Step 2: Do another thing
inside step 2
`)
	assertLogMatchesContent(t, base, logNameStderr, `err in step 2
more err in step 2
`)

	// Ensure that we ended up with both of the expected sub-steps, each
	// with the correct log output.
	// NOTE: We'd like to verify that the stderr lines went to step2 and not
	// step1 below, but due to the racy nature of the two streams described
	// in the docstring for Run(), we cannot guarantee that stderr lines are
	// attached to the correct sub-step.
	require.Equal(t, 2, len(base.Steps))
	step1 := base.Steps[0]
	require.Equal(t, "Do a thing", step1.Name)
	require.Equal(t, td.StepResultSuccess, step1.Result)
	assertLogMatchesContent(t, step1, logNameStdout, `Step 1: Do a thing
log for step 1
... more
`)

	step2 := base.Steps[1]
	require.Equal(t, "Do another thing", step2.Name)
	require.Equal(t, td.StepResultSuccess, step2.Result)
	assertLogMatchesContent(t, step2, logNameStdout, `Step 2: Do another thing
inside step 2
`)
}
