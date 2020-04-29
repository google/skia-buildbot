package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/td"
)

func TestSubprocessExample_UseWithCommandCollector(t *testing.T) {
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mock := exec.CommandCollector{}
		ctx = exec.NewContext(ctx, mock.Run)
		err := SubprocessExample(ctx)
		if err != nil {
			assert.NoError(t, err)
			return err
		}
		require.Len(t, mock.Commands(), 1)
		cmd := exec.DebugString(mock.Commands()[0])
		fmt.Println(cmd)
		assert.Equal(t, "llamasay /tmp/file", cmd)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}
