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
	exit.AssertWasCalledWithCode(t, 0)
	logs := output.String()
	assert.Contains(t, logs, `Falling back to gsutil implementation
This should not be used in production.`)
	b, err := ioutil.ReadFile(filepath.Join(workDir, "auth_opt.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"Luci":false,"ServiceAccount":"","GSUtil":true}`, strings.TrimSpace(string(b)))
}

func runUntilExit(t *testing.T, f func()) {
	assert.Panics(t, f, "panicking signifies an exit, so we expected to see one.")
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
	panic("exited")
}

// AssertWasCalledWithCode make sure ExitWithCode was called previously.
func (e *exitCodeRecorder) AssertWasCalledWithCode(t *testing.T, code int) {
	assert.True(t, e.wasCalled, "Was not called!")
	assert.Equal(t, code, e.code)
}
