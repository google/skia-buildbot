package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/td"
)

// TestSubprocessExample_UseWithCommandCollector shows how to properly tell task driver to use
// a mock implementation of exec for its child subprocesses.
func TestSubprocessExample_UseWithCommandCollector(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mock := exec.CommandCollector{}
		// In other code, this would be exec.NewContext(ctx, mock.Run), but that doesn't work with
		// task driver's setup.
		// TODO(borenet) Could this be done automatically by teaching taskdriver about RunFn?
		ctx = td.WithExecRunFn(ctx, mock.Run)
		err := subprocessExample(ctx)
		if err != nil {
			assert.NoError(t, err)
			return err
		}
		require.Len(t, mock.Commands(), 2)
		cmd := mock.Commands()[0]
		assert.Equal(t, "llamasay", cmd.Name)
		assert.Equal(t, []string{"hello", "world"}, cmd.Args)

		cmd = mock.Commands()[1]
		assert.Equal(t, "bearsay", cmd.Name)
		assert.Equal(t, []string{"good", "night", "moon"}, cmd.Args)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}
