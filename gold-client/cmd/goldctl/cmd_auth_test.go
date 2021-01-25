package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestAuth_WithGSUtil_Success(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()

	env := authEnv{
		flagWorkDir: workDir,
	}
	output := bytes.Buffer{}
	exit := &exitCodeRecorder{}
	ctx := executionContext(context.Background(), &output, &output, exit.ExitWithCode)

	runUntilExit(t, func() {
		env.Auth(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())
	logs := output.String()
	assert.Contains(t, logs, `Falling back to gsutil implementation
This should not be used in production.`)
	b, err := ioutil.ReadFile(filepath.Join(workDir, "auth_opt.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"Luci":false,"ServiceAccount":"","GSUtil":true}`, strings.TrimSpace(string(b)))
}

func setupAuthWithGSUtil(t *testing.T, workDir string) {
	env := authEnv{
		flagWorkDir: workDir,
	}
	devnull := bytes.Buffer{}
	exit := &exitCodeRecorder{}
	ctx := executionContext(context.Background(), &devnull, &devnull, exit.ExitWithCode)
	runUntilExit(t, func() {
		env.Auth(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, devnull.String())
}

const exitSentinel = "exited"

func runUntilExit(t *testing.T, f func()) {
	require.PanicsWithValue(t, exitSentinel, f, "panicking signifies an exit, so we expected to see one.")
}

type exitCodeRecorder struct {
	code      int
	wasCalled bool
}

// ExitWithCode captures the provided code and then panics to stop execution flow (similar to
// how an real exit would work).
func (e *exitCodeRecorder) ExitWithCode(code int) {
	e.code = code
	e.wasCalled = true
	panic(exitSentinel)
}

// AssertWasCalledWithCode make sure ExitWithCode was called previously.
func (e *exitCodeRecorder) AssertWasCalledWithCode(t *testing.T, code int, logs string) {
	require.True(t, e.wasCalled, "exit was not called!: %s", logs)
	require.Equal(t, code, e.code, logs)
}
