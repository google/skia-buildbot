package log_parser

import (
	"bufio"
	"context"
	"errors"
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
		return Run(ctx, ".", []string{"echo", "a b c"}, bufio.ScanWords, func(ctx context.Context, line string) error {
			for _, s := range steps {
				if s.Name == line {
					ctx := td.StartStep(ctx, td.Props(s.Name))
					if s.Result != td.STEP_RESULT_SUCCESS {
						_ = td.FailStep(ctx, errors.New("mock fail"))
					}
					td.EndStep(ctx)
					return nil
				}
			}
			return fmt.Errorf("No matching step for %q", line)
		}, func(ctx context.Context) error {
			ctx = td.StartStep(ctx, td.Props("cleanup"))
			td.EndStep(ctx)
			return nil
		})
	})

	// Verify that we got the expected structure.
	require.Equal(t, 1, len(res.Steps))
	cmdStep := res.Steps[0]
	require.Equal(t, "echo a b c", cmdStep.Name)
	require.Equal(t, len(steps)+1, len(cmdStep.Steps))
	for idx, actualStep := range cmdStep.Steps {
		if idx < len(steps) {
			// Normal steps.
			require.Equal(t, steps[idx].Name, actualStep.Name)
			require.Equal(t, steps[idx].Result, actualStep.Result)
		} else {
			// Cleanup step.
			require.Equal(t, "cleanup", actualStep.Name)
			require.Equal(t, td.STEP_RESULT_SUCCESS, actualStep.Result)
		}
	}
}
