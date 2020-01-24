package test2json

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	// We use actual Go tests instead of just mocking output and parsing it
	// so that if the output format changes our tests will catch it.
	tmpl = `package %s

import "testing"

func %s(t *testing.T) {
%s
}
`
	ModulePath      = "fake.com/test2json_test"
	PackageName     = "test2json_test"
	PackageFullPath = ModulePath + "/go/" + PackageName
	TestName        = "TestCase"
	FailText        = "the test failed"
	PassText        = "the test passed"
	SkipText        = "no thanks!"
)

var (
	CONTENT_FAIL TestContent = []byte(fmt.Sprintf(tmpl, PackageName, TestName, fmt.Sprintf("t.Fatalf(%q)", FailText)))
	CONTENT_PASS TestContent = []byte(fmt.Sprintf(tmpl, PackageName, TestName, fmt.Sprintf("t.Log(%q)", PassText)))
	CONTENT_SKIP TestContent = []byte(fmt.Sprintf(tmpl, PackageName, TestName, fmt.Sprintf("t.Skip(%q)", SkipText)))

	EVENTS_FAIL = []*Event{
		{
			Action:  ACTION_RUN,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- FAIL: %s (0.00s)\n", TestName),
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("    test2json_test.go:6: the test failed\n"),
		},
		{
			Action:  ACTION_FAIL,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("FAIL\n"),
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("FAIL\t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ACTION_FAIL,
			Package: PackageFullPath,
		},
	}

	EVENTS_PASS = []*Event{
		{
			Action:  ACTION_RUN,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- PASS: %s (0.00s)\n", TestName),
		},
		{
			Action:  ACTION_PASS,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Output:  "PASS\n",
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("ok  \t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ACTION_PASS,
			Package: PackageFullPath,
		},
	}

	EVENTS_SKIP = []*Event{
		{
			Action:  ACTION_RUN,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("=== RUN   %s\n", TestName),
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("--- SKIP: %s (0.00s)\n", TestName),
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  "    test2json_test.go:6: no thanks!\n",
		},
		{
			Action:  ACTION_SKIP,
			Package: PackageFullPath,
			Test:    TestName,
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Output:  "PASS\n",
		},
		{
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Output:  fmt.Sprintf("ok  \t%s\t0.00s\n", PackageFullPath),
		},
		{
			Action:  ACTION_PASS,
			Package: PackageFullPath,
		},
	}

	tsRegex = regexp.MustCompile(`\d+\.\d+s`)
)

type TestContent []byte

func SetupTest(t sktest.TestingT, content TestContent) (string, func()) {
	// Create a temporary dir with a go package to test.
	tmp, cleanup := testutils.TempDir(t)
	pkgPath := filepath.Join(tmp, "go", "test2json_test")
	require.NoError(t, os.MkdirAll(pkgPath, os.ModePerm))
	require.NoError(t, ioutil.WriteFile(filepath.Join(pkgPath, "test2json_test.go"), content, os.ModePerm))

	// Make go modules happy.
	ctx := context.Background()
	_, err := exec.RunCwd(ctx, tmp, "go", "mod", "init", "fake.com/test2json_test")
	require.NoError(t, err)

	return tmp, cleanup
}

func RunTest(t sktest.TestingT, w io.Writer, content TestContent) {
	// Setup.
	testDir, cleanup := SetupTest(t, content)
	defer cleanup()

	// Ignore the error, since some cases expect it.
	_, _ = exec.RunCommand(context.Background(), &exec.Command{
		Name:   "go",
		Args:   []string{"test", "-json", "./..."},
		Dir:    testDir,
		Stdout: w,
	})
}

func RunTestAndCompare(t sktest.TestingT, expectEvents []*Event, content TestContent) {
	r, w := io.Pipe()
	go func() {
		defer testutils.AssertCloses(t, w)
		RunTest(t, w, content)
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
