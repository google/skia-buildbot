package log_parser

import (
	"bufio"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
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
