package test2json

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

var (
	eventsFail = []*Event{
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("    test2json_test.go:6: %s\n", FailText),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- FAIL: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionFail,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("FAIL\n"),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("FAIL\t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ActionFail,
			Package: PackageFullPath,
		},
	}

	eventsPass = []*Event{
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("    test2json_test.go:6: %s\n", passText),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- PASS: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  "PASS\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("ok  \t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
		},
	}

	eventsSkip = []*Event{
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  "    test2json_test.go:6: no thanks!\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- SKIP: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionSkip,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  "PASS\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("ok  \t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
		},
	}

	eventsNested = []*Event{
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  "    test2json_test.go:6: test-level log, before sub-steps\n",
		},
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName + "/1",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1",
			Output:  fmt.Sprintf("=== RUN   %s/1\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1",
			Output:  "    test2json_test.go:8: nested 1 log, before sub-steps\n",
		},
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
			Output:  fmt.Sprintf("=== RUN   %s/1/2\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
			Output:  "    test2json_test.go:10: nested 2 log, before sub-steps\n",
		},
		{
			Action:  ActionRun,
			Package: PackageFullPath,
			Test:    TestName + "/1/2/3",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2/3",
			Output:  fmt.Sprintf("=== RUN   %s/1/2/3\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2/3",
			Output:  "    test2json_test.go:12: nested 3 log\n",
		},
		{
			Action:  ActionCont,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
			Output:  fmt.Sprintf("=== CONT  %s/1/2\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
			Output:  "    test2json_test.go:14: nested 2 log, after sub-steps\n",
		},
		{
			Action:  ActionCont,
			Package: PackageFullPath,
			Test:    TestName + "/1",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1",
			Output:  fmt.Sprintf("=== CONT  %s/1\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1",
			Output:  "    test2json_test.go:16: nested 1 log, after sub-steps\n",
		},
		{
			Action:  ActionCont,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== CONT  %s\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  "    test2json_test.go:18: test-level log, after sub-steps\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- PASS: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1",
			Output:  fmt.Sprintf("    --- PASS: %s/1 (0.00s)\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
			Output:  fmt.Sprintf("        --- PASS: %s/1/2 (0.00s)\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName + "/1/2/3",
			Output:  fmt.Sprintf("            --- PASS: %s/1/2/3 (0.00s)\n", TestName),
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
			Test:    TestName + "/1/2/3",
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
			Test:    TestName + "/1/2",
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
			Test:    TestName + "/1",
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  "PASS\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("ok  \t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ActionPass,
			Package: PackageFullPath,
		},
	}

	tsRegex = regexp.MustCompile(`\d+\.\d+s`)
)

func runTest(t sktest.TestingT, w io.Writer, content TestContent) {
	// For compatibility with Bazel: the "go" command fails if HOME is not set.
	testutils.SetUpFakeHomeDir(t.(*testing.T), "test2json_test")

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
		require.Equal(t, expect, actual)
		i++
	}
}

func TestEventStreamFail(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, eventsFail, ContentFail)
}

func TestEventStreamPass(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, eventsPass, ContentPass)
}

func TestEventStreamSkip(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, eventsSkip, ContentSkip)
}

func TestEventStreamNested(t *testing.T) {
	unittest.MediumTest(t)
	runTestAndCompare(t, eventsNested, ContentNested)
}
