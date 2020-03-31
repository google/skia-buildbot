package test2json

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/util"
)

const (
	// We use actual Go tests instead of just mocking output and parsing it
	// so that if the output format changes our tests will catch it.
	ModulePath      = "fake.com/test2json_test"
	PackageName     = "test2json_test"
	PackageFullPath = ModulePath + "/go/" + PackageName
	TestName        = "TestCase"
	FailText        = "the test failed"
	PassText        = "the test passed"
	SkipText        = "no thanks!"

	ContentFail TestContent = `package test2json_test

import "testing"

func TestCase(t *testing.T) {
	t.Fatalf("the test failed")
}`
	ContentPass TestContent = `package test2json_test

import "testing"

func TestCase(t *testing.T) {
	t.Log("the test passed")
}`
	ContentSkip TestContent = `package test2json_test

import "testing"

func TestCase(t *testing.T) {
	t.Skip("no thanks!")
}`
	ContentNested TestContent = `package test2json_test

import (
	"fmt"
	"testing"
)

func TestCase(t *testing.T) {
	t.Logf("test-level log, before sub-steps")
	t.Run("1", func(t *testing.T) {
		t.Logf("nested 1 log, before sub-steps")
		t.Run("2", func(t *testing.T) {
			t.Logf("nested 2 log, before sub-steps")
			t.Run("3", func(t *testing.T) {
				t.Logf("nested 3 log")
			})
			t.Logf("nested 2 log, after sub-steps")
		})
		t.Logf("nested 1 log, after sub-steps")
	})
	t.Logf("test-level log, after sub-steps")
}`
)

var (
	EventsFail = []*Event{
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
			Output:  fmt.Sprintf("--- FAIL: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("    test2json_test.go:6: %s\n", FailText),
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

	EventsPass = []*Event{
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
			Output:  fmt.Sprintf("--- PASS: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("    test2json_test.go:6: %s\n", PassText),
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

	EventsSkip = []*Event{
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
			Output:  fmt.Sprintf("--- SKIP: %s (0.00s)\n", TestName),
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  "    test2json_test.go:6: no thanks!\n",
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

	EventsNested = []*Event{
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
			Output:  "test-level log, before sub-steps\n",
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
			Output:  "nested 1 log, before sub-steps\n",
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
			Output:  "nested 2 log, before sub-steps\n",
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
			Output:  "nested 3 log\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			// Note: Unfortunately, it seems that output is
			// attributed to the most recently started sub-test,
			// despite using t.Log() on the testing.T instance for
			// a specific sub-step.
			Test:   TestName + "/1/2/3",
			Output: "nested 2 log, after sub-steps\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			// Note: Unfortunately, it seems that output is
			// attributed to the most recently started sub-test,
			// despite using t.Log() on the testing.T instance for
			// a specific sub-step.
			Test:   TestName + "/1/2/3",
			Output: "nested 1 log, after sub-steps\n",
		},
		{
			Action:  ActionOutput,
			Package: PackageFullPath,
			// Note: Unfortunately, it seems that output is
			// attributed to the most recently started sub-test,
			// despite using t.Log() on the testing.T instance for
			// a specific sub-step.
			Test:   TestName + "/1/2/3",
			Output: "test-level log, after sub-steps\n",
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

type TestContent string

// SetupTest sets up a temporary directory containing a test file with the given
// content so that the caller may run `go test` in the returned directory.
// Returns the directory path and a cleanup function, or any error which
// occurred. SetupTest is intended to be used with any of the CONTENT provided
// above, in which case EventStream/ParseEvent should generate the corresponding
// sequence of EVENTS from above.
func SetupTest(content TestContent) (tmpDir string, cleanup func(), err error) {
	// Create a temporary dir with a go package to test.
	tmpDir, err = ioutil.TempDir("", "")
	if err != nil {
		return
	}
	cleanup = func() {
		util.RemoveAll(tmpDir)
	}
	defer func() {
		if err != nil {
			cleanup()
			cleanup = nil
		}
	}()
	pkgPath := filepath.Join(tmpDir, "go", "test2json_test")
	err = os.MkdirAll(pkgPath, os.ModePerm)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filepath.Join(pkgPath, "test2json_test.go"), []byte(content), os.ModePerm)
	if err != nil {
		return
	}

	// Make go modules happy.
	ctx := context.Background()
	_, err = exec.RunCwd(ctx, tmpDir, "go", "mod", "init", "fake.com/test2json_test")
	return
}
