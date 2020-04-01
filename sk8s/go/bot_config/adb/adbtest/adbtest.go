// Package adbtest contains utilities for testing adb.
package adbtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/exec"
)

// AdbMockHappy returns a context that mocks out a response when calling exec.Run().
func AdbMockHappy(t *testing.T, response string) context.Context {
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		_, err := cmd.Stdout.Write([]byte(response))
		assert.NoError(t, err)
		return nil
	})
	return exec.NewContext(context.Background(), mock.Run)
}

// AdbMockError returns a context that mocks out an error when calling exec.Run().
//
// Also mocks out the stderr output from adb.
func AdbMockError(t *testing.T, stderr string) context.Context {
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		_, err := cmd.Stderr.Write([]byte(stderr))
		assert.NoError(t, err)
		return fmt.Errorf("exit code 1")

	})
	return exec.NewContext(context.Background(), mock.Run)
}
