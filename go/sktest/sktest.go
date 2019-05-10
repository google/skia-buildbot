package sktest

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/stretchr/testify/assert"
)

// TestingT is an interface which is compatible with testing.T and testing.B,
// used so that we don't have to import the "testing" package except in _test.go
// files.
type TestingT interface {
	Error(...interface{})
	Errorf(string, ...interface{})
	Fail()
	FailNow()
	Failed() bool
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Helper()
	Log(...interface{})
	Logf(string, ...interface{})
	Name() string
	Skip(...interface{})
	SkipNow()
	Skipf(string, ...interface{})
	Skipped() bool
}

// AssertCloses takes an ioutil.Closer and asserts that it closes. E.g.:
// frobber := NewFrobber()
// defer testutils.AssertCloses(t, frobber)
func AssertCloses(t TestingT, c io.Closer) {
	assert.NoError(t, c.Close())
}

// Remove attempts to remove the given file and asserts that no error is returned.
func Remove(t TestingT, fp string) {
	assert.NoError(t, os.Remove(fp))
}

// RemoveAll attempts to remove the given directory and asserts that no error is returned.
func RemoveAll(t TestingT, fp string) {
	assert.NoError(t, os.RemoveAll(fp))
}

// TempDir is a wrapper for ioutil.TempDir. Returns the path to the directory and a cleanup
// function to defer.
func TempDir(t TestingT) (string, func()) {
	d, err := ioutil.TempDir("", "testutils")
	assert.NoError(t, err)
	return d, func() {
		RemoveAll(t, d)
	}
}
