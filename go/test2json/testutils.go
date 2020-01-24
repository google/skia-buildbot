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
			Output:  fmt.Sprintf("    test2json_test.go:6: %s\n", FailText),
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
			Action:  ACTION_OUTPUT,
			Package: PackageFullPath,
			Test:    TestName,
			Output:  fmt.Sprintf("    test2json_test.go:6: %s\n", PassText),
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
	err = ioutil.WriteFile(filepath.Join(pkgPath, "test2json_test.go"), content, os.ModePerm)
	if err != nil {
		return
	}

	// Make go modules happy.
	ctx := context.Background()
	_, err = exec.RunCwd(ctx, tmpDir, "go", "mod", "init", "fake.com/test2json_test")
	return
}
