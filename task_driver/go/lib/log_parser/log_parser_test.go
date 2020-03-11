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
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/td"
)

func TestRun(t *testing.T) {
	unittest.MediumTest(t)

	// Create a dummy output format which just indicates results of steps.
	steps := []struct {
		Name   string
		Result td.StepResult
	}{
		{
			Name:   "a",
			Result: td.STEP_RESULT_SUCCESS,
		},
		{
			Name:   "b",
			Result: td.STEP_RESULT_FAILURE,
		},
		{
			Name:   "c",
			Result: td.STEP_RESULT_SUCCESS,
		},
	}
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Run(ctx, ".", []string{"echo", "a b c"}, bufio.ScanWords, func(sm *StepManager, line string) error {
			for _, stepData := range steps {
				if stepData.Name == line {
					s := sm.StartStep(td.Props(stepData.Name))
					if stepData.Result != td.STEP_RESULT_SUCCESS {
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

func TestTimeout(t *testing.T) {
	unittest.MediumTest(t)
	// TODO(borenet): This test will not work on Windows.

	// Write a script to generate steps.
	tmp, cleanup := testutils.TempDir(t)
	defer cleanup()
	script := filepath.Join(tmp, "script.sh")
	testutils.WriteFile(t, script, `#!/bin/bash
echo "Step1"
echo "Step2"
sleep 10
`)

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		timeout := 100 * time.Millisecond
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		start := time.Now()
		defer func() {
			elapsed := time.Now().Sub(start)
			require.True(t, elapsed < 2*time.Second, fmt.Sprintf("Timeout is %s; command exited after %s", timeout, elapsed))
		}()
		return Run(ctx, ".", []string{script}, bufio.ScanLines, func(sm *StepManager, line string) error {
			s := sm.CurrentStep()
			if s != nil {
				s.End()
			}
			sm.StartStep(td.Props(line))
			return nil
		})
	})
	require.Equal(t, 1, len(res.Steps))
	require.Equal(t, td.STEP_RESULT_FAILURE, res.Steps[0].Result)
	require.Equal(t, 2, len(res.Steps[0].Steps))
	require.Equal(t, td.STEP_RESULT_SUCCESS, res.Steps[0].Steps[0].Result)
	require.Equal(t, td.STEP_RESULT_FAILURE, res.Steps[0].Steps[1].Result)
}

func TestLogs(t *testing.T) {
	unittest.MediumTest(t)

	// Write a script to generate steps.
	tmp, cleanup := testutils.TempDir(t)
	defer cleanup()
	script := filepath.Join(tmp, "script.py")
	testutils.WriteFile(t, script, `
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
	stepRe := regexp.MustCompile(`^Step \d: (.+)$`)
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		var activeStep *Step
		return Run(ctx, ".", []string{"python", "-u", script}, bufio.ScanLines, func(sm *StepManager, line string) error {
			m := stepRe.FindStringSubmatch(line)
			if len(m) > 1 {
				if activeStep != nil {
					activeStep.End()
				}
				activeStep = sm.StartStep(td.Props(m[1]))
			}
			if activeStep != nil {
				activeStep.Stdout(line)
			}
			return nil
		})
	})

	// testLog verifies that the given log buffer contains the given lines.
	testLog := func(s *td.StepReport, logName, expect string) {
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

	require.Equal(t, 1, len(res.Steps))
	base := res.Steps[0]
	require.Equal(t, td.STEP_RESULT_SUCCESS, base.Result)
	testLog(base, "stdout", `Step 1: Do a thing
log for step 1
... more
Step 2: Do another thing
inside step 2
`)
	testLog(base, "stderr", `err in step 2
more err in step 2
`)

	require.Equal(t, 2, len(base.Steps))
	step1 := base.Steps[0]
	require.Equal(t, "Do a thing", step1.Name)
	require.Equal(t, td.STEP_RESULT_SUCCESS, step1.Result)
	testLog(step1, "stdout", `Step 1: Do a thing
log for step 1
... more
`)

	step2 := base.Steps[1]
	require.Equal(t, "Do another thing", step2.Name)
	require.Equal(t, td.STEP_RESULT_SUCCESS, step2.Result)
	testLog(step2, "stdout", `Step 2: Do another thing
inside step 2
`)
}
