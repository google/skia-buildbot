// Convenience utilities for testing.
package testutils

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"runtime"
	"testing"

	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"
)

const (
	SMALL_TEST  = "small"
	MEDIUM_TEST = "medium"
	LARGE_TEST  = "large"
)

var (
	small  = flag.Bool(SMALL_TEST, false, "Whether or not to run small tests.")
	medium = flag.Bool(MEDIUM_TEST, false, "Whether or not to run medium tests.")
	large  = flag.Bool(LARGE_TEST, false, "Whether or not to run large tests.")

	DEFAULT_RUN = map[string]bool{
		SMALL_TEST:  true,
		MEDIUM_TEST: true,
		LARGE_TEST:  false,
	}
)

// shouldRun determines whether the test should run based on the provided flags.
func shouldRun(testType string) bool {
	// Fallback if no test filter is specified.
	if !*small && !*medium && !*large {
		return DEFAULT_RUN[testType]
	}

	switch testType {
	case SMALL_TEST:
		return *small
	case MEDIUM_TEST:
		return *medium
	case LARGE_TEST:
		return *large
	}
	return false
}

// SmallTest is a function which should be called at the beginning of a small
// test: A test (under 2 seconds) with no dependencies on external databases,
// networks, etc.
func SmallTest(t *testing.T) {
	if !shouldRun(SMALL_TEST) {
		t.Skip("Not running small tests.")
	}
}

// MediumTest is a function which should be called at the beginning of an
// medium-sized test: a test (2-15 seconds) which has dependencies on external
// databases, networks, etc.
func MediumTest(t *testing.T) {
	if !shouldRun(MEDIUM_TEST) {
		t.Skip("Not running medium tests.")
	}
}

// LargeTest is a function which should be called at the beginning of a large
// test: a test (> 15 seconds) with significant reliance on external
// dependencies which makes it too slow or flaky to run as part of the normal
// test suite.
func LargeTest(t *testing.T) {
	if !shouldRun(LARGE_TEST) {
		t.Skip("Not running large tests.")
	}
}

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

// WriteFile writes the given contents to the given file path, reporting any
// error.
func WriteFile(t assert.TestingT, filename, contents string) {
	assert.NoError(t, ioutil.WriteFile(filename, []byte(contents), os.ModePerm))
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

// MarshalJSON encodes the given interface to a JSON string.
func MarshalJSON(t *testing.T, i interface{}) string {
	b, err := json.Marshal(i)
	assert.NoError(t, err)
	return string(b)
}
