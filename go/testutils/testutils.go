// Convenience utilities for testing.
package testutils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"runtime"
	"testing"

	"go.skia.org/infra/go/exec"

	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"
)

// SkipIfShort causes the test to be skipped when running with -short.
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test with -short")
	}
}

// AssertDeepEqual fails the test if the two objects do not pass reflect.DeepEqual.
func AssertDeepEqual(t *testing.T, a, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		assert.FailNow(t, fmt.Sprintf("Objects do not match: \na:\n%s\n\nb:\n%s\n", spew.Sprint(a), spew.Sprint(b)))
	}
}

// AssertCopy is AssertDeepEqual but also checks that none of the direct fields
// have a zero value and none of the direct fields point to the same object.
// This catches regressions where a new field is added without adding that field
// to the Copy method. Arguments must be structs.
func AssertCopy(t *testing.T, a, b interface{}) {
	AssertDeepEqual(t, a, b)

	// Check that all fields are non-zero.
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	assert.Equal(t, va.Type(), vb.Type(), "Arguments are different types.")
	for va.Kind() == reflect.Ptr {
		assert.Equal(t, reflect.Ptr, vb.Kind(), "Arguments are different types (pointer vs. non-pointer)")
		va = va.Elem()
		vb = vb.Elem()
	}
	assert.Equal(t, reflect.Struct, va.Kind(), "Not a struct or pointer to struct.")
	assert.Equal(t, reflect.Struct, vb.Kind(), "Arguments are different types (pointer vs. non-pointer)")
	for i := 0; i < va.NumField(); i++ {
		fa := va.Field(i)
		z := reflect.Zero(fa.Type())
		if reflect.DeepEqual(fa.Interface(), z.Interface()) {
			assert.FailNow(t, fmt.Sprintf("Missing field %q (or set to zero value).", va.Type().Field(i).Name))
		}
		if fa.Kind() == reflect.Map || fa.Kind() == reflect.Ptr || fa.Kind() == reflect.Slice {
			fb := vb.Field(i)
			assert.NotEqual(t, fa.Pointer(), fb.Pointer(), "Field %q not deep-copied.", va.Type().Field(i).Name)
		}
	}
}

// TestDataDir returns the path to the caller's testdata directory, which
// is assumed to be "<path to caller dir>/testdata".
func TestDataDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("Could not find test data dir: runtime.Caller() failed.")
	}
	for skip := 0; ; skip++ {
		_, file, _, ok := runtime.Caller(skip)
		if !ok {
			return "", fmt.Errorf("Could not find test data dir: runtime.Caller() failed.")
		}
		if file != thisFile {
			return path.Join(path.Dir(file), "testdata"), nil
		}
	}
}

func readFile(filename string) (io.Reader, error) {
	dir, err := TestDataDir()
	if err != nil {
		return nil, fmt.Errorf("Could not read %s: %v", filename, err)
	}
	f, err := os.Open(path.Join(dir, filename))
	if err != nil {
		return nil, fmt.Errorf("Could not read %s: %v", filename, err)
	}
	return f, nil
}

// ReadFile reads a file from the caller's testdata directory.
func ReadFile(filename string) (string, error) {
	f, err := readFile(filename)
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("Could not read %s: %v", filename, err)
	}
	return string(b), nil
}

// MustReadFile reads a file from the caller's testdata directory and panics on
// error.
func MustReadFile(filename string) string {
	s, err := ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return s
}

// ReadJsonFile reads a JSON file from the caller's testdata directory into the
// given interface.
func ReadJsonFile(filename string, dest interface{}) error {
	f, err := readFile(filename)
	if err != nil {
		return err
	}
	return json.NewDecoder(f).Decode(dest)
}

// MustReadJsonFile reads a JSON file from the caller's testdata directory into
// the given interface and panics on error.
func MustReadJsonFile(filename string, dest interface{}) {
	if err := ReadJsonFile(filename, dest); err != nil {
		panic(err)
	}
}

// CloseInTest takes an ioutil.Closer and Closes it, reporting any error.
func CloseInTest(t assert.TestingT, c io.Closer) {
	if err := c.Close(); err != nil {
		t.Errorf("Failed to Close(): %v", err)
	}
}

// AssertCloses takes an ioutil.Closer and asserts that it closes.
func AssertCloses(t assert.TestingT, c io.Closer) {
	assert.NoError(t, c.Close())
}

// Remove attempts to remove the given file and asserts that no error is returned.
func Remove(t assert.TestingT, fp string) {
	assert.NoError(t, os.Remove(fp))
}

// RemoveAll attempts to remove the given directory and asserts that no error is returned.
func RemoveAll(t assert.TestingT, fp string) {
	assert.NoError(t, os.RemoveAll(fp))
}

// Run runs the given command in the given dir and asserts that it succeeds.
func Run(t assert.TestingT, dir string, cmd ...string) {
	_, err := exec.RunCwd(dir, cmd...)
	assert.NoError(t, err)
}
