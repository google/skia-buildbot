package test2json

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func runTest(t sktest.TestingT, w io.Writer, content TestContent) {
	// Setup.
	testDir, cleanup, err := SetupTest(content)
	require.NoError(t, err)
	defer cleanup()

	// Ignore the error, since some cases expect it.
	_, _ = exec.RunCommand(context.Background(), &exec.Command{
		Name:   "go",
		Args:   []string{"test", "-json", "./..."},
		Dir:    testDir,
		Stdout: w,
	})
}

func runTestAndCompare(t sktest.TestingT, expectEvents []*Event, content TestContent) {
	r, w := io.Pipe()
	go func() {
		defer testutils.AssertCloses(t, w)
		runTest(t, w, content)
	}()
	i := 0
	for actual := range EventStream(r) {
		expect := expectEvents[i]

		// Fake out some fields.
		require.False(t, util.TimeIsZero(actual.Time))
		actual.Time = expect.Time
		actual.Output = tsRegex.ReplaceAllString(actual.Output, "0.00s")
		actual.Elapsed = 0.0

		// Compare to the expected event.
		sklog.Errorf("Event %d", i)
		assertdeep.Equal(t, expect, actual)
		i++
	}
}

func TestEventStreamFail(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, EVENTS_FAIL, CONTENT_FAIL)
}

func TestEventStreamPass(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, EVENTS_PASS, CONTENT_PASS)
}

func TestEventStreamSkip(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, EVENTS_SKIP, CONTENT_SKIP)
}
