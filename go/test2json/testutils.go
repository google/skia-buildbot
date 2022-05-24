package test2json

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/golang"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	// We use actual Go tests instead of just mocking output and parsing it
	// so that if the output format changes our tests will catch it.
	modulePath      = "fake.com/test2json_test"
	packageName     = "test2json_test"
	PackageFullPath = modulePath + "/go/" + packageName
	TestName        = "TestCase"
	FailText        = "the test failed"
	passText        = "the test passed"
	skipText        = "no thanks!"

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

import "testing"

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

type TestContent string

// SetupTest sets up a temporary directory containing a test file with the given
// content so that the caller may run `go test` in the returned directory.
// Returns the directory path and a cleanup function, or any error which
// occurred. SetupTest is intended to be used with any of the Content provided
// above, in which case EventStream/ParseEvent should generate the corresponding
// sequence of Events from above.
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
	var goBin string
	goBin, err = golang.FindGo()
	if err != nil {
		err = skerr.Wrap(err)
		return
	}
	_, err = exec.RunCwd(ctx, tmpDir, goBin, "mod", "init", "fake.com/test2json_test")
	return
}
