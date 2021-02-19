// Package testutils contains convenience utilities for testing.
package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/util"
)

var (
	// TryAgainErr use used by TryUntil.
	TryAgainErr = errors.New("Trying Again")
)

// TestDataDir returns the path to the caller's testdata directory, which
// is assumed to be "<path to caller dir>/testdata".
func TestDataDir(t sktest.TestingT) string {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "Could not find test data dir: runtime.Caller() failed.")
	for skip := 0; ; skip++ {
		_, file, _, ok := runtime.Caller(skip)
		require.True(t, ok, "Could not find test data dir: runtime.Caller() failed.")
		if file != thisFile {
			// Under Bazel, the path returned by runtime.Caller() is relative to the workspace's root
			// directory (e.g. "go/testutils"). We prepend this with the absolute path to the runfiles
			// directory so that tests can find these files with no further changes.
			//
			// Under "go test" this is not necessary because the path returned by runtime.Caller() is
			// absolute.
			if bazel.InBazel() {
				file = filepath.Join(bazel.RunfilesDir(), file)
			}

			return filepath.Join(filepath.Dir(file), "testdata")
		}
	}
}

// ReadFileBytes reads a file from the caller's testdata directory and returns its contents as a
// slice of bytes.
func ReadFileBytes(t sktest.TestingT, filename string) []byte {
	f := GetReader(t, filename)
	b, err := ioutil.ReadAll(f)
	require.NoError(t, err, "Could not read %s: %v", filename)
	require.NoError(t, f.Close())
	return b
}

// ReadFile reads a file from the caller's testdata directory.
func ReadFile(t sktest.TestingT, filename string) string {
	b := ReadFileBytes(t, filename)
	return string(b)
}

// GetReader reads a file from the caller's testdata directory and panics on
// error.
func GetReader(t sktest.TestingT, filename string) io.ReadCloser {
	dir := TestDataDir(t)
	f, err := os.Open(filepath.Join(dir, filename))
	require.NoError(t, err, "Reading %s from testdir", filename)
	return f
}

// ReadJSONFile reads a JSON file from the caller's testdata directory into the
// given interface.
func ReadJSONFile(t sktest.TestingT, filename string, dest interface{}) {
	f := GetReader(t, filename)
	err := json.NewDecoder(f).Decode(dest)
	require.NoError(t, err, "Decoding JSON in %s", filename)
	require.NoError(t, f.Close())
}

// WriteFile writes the given contents to the given file path, reporting any
// error.
func WriteFile(t sktest.TestingT, filename, contents string) {
	require.NoErrorf(t, ioutil.WriteFile(filename, []byte(contents), os.ModePerm), "Unable to write to file %s", filename)
}

// AssertCloses takes an ioutil.Closer and asserts that it closes. E.g.:
// frobber := NewFrobber()
// defer testutils.AssertCloses(t, frobber)
func AssertCloses(t sktest.TestingT, c io.Closer) {
	require.NoError(t, c.Close())
}

// Remove attempts to remove the given file and asserts that no error is returned.
func Remove(t sktest.TestingT, fp string) {
	require.NoError(t, os.Remove(fp))
}

// RemoveAll attempts to remove the given directory and asserts that no error is returned.
func RemoveAll(t sktest.TestingT, fp string) {
	require.NoError(t, os.RemoveAll(fp))
}

// TempDir is a wrapper for ioutil.TempDir. Returns the path to the directory and a cleanup
// function to defer.
// TODO(kjlubick) replace this with testing.TempDir()
func TempDir(t sktest.TestingT) (string, func()) {
	d, err := ioutil.TempDir("", "testutils")
	require.NoError(t, err)
	return d, func() {
		RemoveAll(t, d)
	}
}

// MarshalJSON encodes the given interface to a JSON string.
func MarshalJSON(t sktest.TestingT, i interface{}) string {
	b, err := json.Marshal(i)
	require.NoError(t, err)
	return string(b)
}

// MarshalIndentJSON encodes the given interface to an indented JSON string.
func MarshalIndentJSON(t sktest.TestingT, i interface{}) string {
	b, err := json.MarshalIndent(i, "", "  ")
	require.NoError(t, err)
	return string(b)
}

// GetRepoRoot returns the path to the root of the checkout.
func GetRepoRoot(t sktest.TestingT) string {
	root, err := repo_root.Get()
	require.NoError(t, err)
	return root
}

// EventuallyConsistent tries a test repeatedly until either the test passes
// or time expires, and is used when tests are written to expect
// non-eventual consistency.
//
// Use this function sparingly.
//
// duration - The amount of time to keep trying.
// f - The func to run the tests, should return TryAgainErr if
//     we should keep trying, otherwise TryUntil will return
//     with the err that f() returns.
func EventuallyConsistent(duration time.Duration, f func() error) error {
	begin := time.Now()
	for time.Now().Sub(begin) < duration {
		if err := f(); err != TryAgainErr {
			return err
		}
	}
	return fmt.Errorf("Failed to pass test in allotted time.")
}

// MockTestingT implements sktest.TestingT by saving calls to Log and Fail. MockTestingT can
// be used to test a test helper function. See also AssertFails.
// The methods Helper, Name, Skip, SkipNow, Skipf, and Skipped are unimplemented.
// This type is not safe for concurrent use.
type MockTestingT struct {
	LogMsgs  []string
	IsFailed bool
}

func (m *MockTestingT) Cleanup(fn func()) {
	panic("Cleanup is not implemented.")
}
func (m *MockTestingT) Error(args ...interface{}) {
	m.Log(args...)
	m.Fail()
}
func (m *MockTestingT) Errorf(format string, args ...interface{}) {
	m.Logf(format, args...)
	m.Fail()
}
func (m *MockTestingT) Fail() {
	m.IsFailed = true
}
func (m *MockTestingT) FailNow() {
	m.Fail()
	runtime.Goexit()
}
func (m *MockTestingT) Failed() bool {
	return m.IsFailed
}
func (m *MockTestingT) Fatal(args ...interface{}) {
	m.Log(args...)
	m.FailNow()
}
func (m *MockTestingT) Fatalf(format string, args ...interface{}) {
	m.Logf(format, args...)
	m.FailNow()
}
func (m *MockTestingT) Helper() {}
func (m *MockTestingT) Log(args ...interface{}) {
	m.LogMsgs = append(m.LogMsgs, fmt.Sprintln(args...))
}
func (m *MockTestingT) Logf(format string, args ...interface{}) {
	m.LogMsgs = append(m.LogMsgs, fmt.Sprintf(format, args...))
}
func (m *MockTestingT) Name() string {
	return ""
}
func (m *MockTestingT) Skip(args ...interface{}) {
	m.Log(args...)
	m.SkipNow()
}
func (m *MockTestingT) SkipNow() {
	panic("SkipNow is not implemented.")
}
func (m *MockTestingT) Skipf(format string, args ...interface{}) {
	m.Logf(format, args...)
	m.SkipNow()
}
func (m *MockTestingT) Skipped() bool {
	return false
}

// Assert that MockTestingT implements the sktest.TestingT interface:
var _ sktest.TestingT = (*MockTestingT)(nil)

// AssertFails runs testfn with a MockTestingT and asserts that the test fails and the first failure
// logged matches the regexp. The sktest.TestingT passed to testfn is not safe for concurrent use.
func AssertFails(parent sktest.TestingT, regexp string, testfn func(sktest.TestingT)) {
	mock := MockTestingT{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		testfn(&mock)
	}()
	wg.Wait()
	require.True(parent, mock.Failed(), "In AssertFails, the test function did not fail.")
	require.True(parent, len(mock.LogMsgs) > 0, "In AssertFails, the test function did not produce any failure messages.")
	require.Regexp(parent, regexp, mock.LogMsgs[0])
}

// AnyContext can be used to match any Context objects e.g.
// m.On("Foo", testutils.AnyContext).Return(...)
// This is better than trying to used mock.AnythingOfTypeArgument
// because that only works for concrete types, which could be brittle
// (e.g. a "normal" context is *context.emptyCtx, but one modified by
// trace.StartSpan() could be a *context.valueCtx)
var AnyContext = mock.MatchedBy(func(c context.Context) bool {
	// if the passed in parameter does not implement the context.Context interface, the
	// wrapping MatchedBy will panic - so we can simply return true, since we
	// know it's a context.Context if execution flow makes it here.
	return true
})

// ExecTemplate parses the given string as a text template, executes it using
// the given data, and returns the result as a string.
func ExecTemplate(t sktest.TestingT, tmpl string, data interface{}) string {
	template, err := template.New(uuid.New().String()).Parse(tmpl)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, template.Execute(&buf, data))
	return buf.String()
}

// SetUpFakeHomeDir creates a temporary dir and updates the HOME environment variable with its path.
// After the caller test completes, the HOME environment variable will be restored to its original
// value, and the temporary dir will be deleted.
//
// Under Bazel, this is useful because Bazel does not set the HOME environment variable. Without
// this, some tests that call the "go" binary fail, because some "go" subcommands create a cache
// under $HOME/.cache/go-build.
//
// Outside of Bazel (i.e. "go test"), this is still useful because it leads to more hermetic tests
// which do not depend on the specifics of the $HOME directory in the host system.
//
// See https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions.
func SetUpFakeHomeDir(t sktest.TestingT, tempDirPattern string) {
	fakeHome, err := ioutil.TempDir("", tempDirPattern)
	require.NoError(t, err)
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", fakeHome))

	t.Cleanup(func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
		util.RemoveAll(fakeHome)
	})
}
