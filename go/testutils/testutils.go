// Convenience utilities for testing.
package testutils

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"skia.googlesource.com/buildbot.git/go/gs"
	"skia.googlesource.com/buildbot.git/go/util"
)

const (
	// GS bucket where we store test data. Add a folder to this bucket
	// with the tests for a particular component.
	GS_TEST_DATA_ROOT_URI = "http://storage.googleapis.com/skia-infra-testdata/"
)

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

// DownloadTestDataFile downloads a file with test data from Google Storage.
// The uriPath identifies what to download from the test bucket in GS.
// The content must be publicly accessible.
// The file will be downloaded and stored at provided target
// path (regardless of what the original name is).
// If the the uri ends with '.gz' it will be transparently unzipped.
func DownloadTestDataFile(uriPath, targetPath string) error {
	uri := GS_TEST_DATA_ROOT_URI + uriPath

	dir, _ := filepath.Split(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	client := util.NewTimeoutClient()
	request, err := gs.RequestForStorageURL(uri)
	if err != nil {
		return err
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Downloading %s failed. Got response status: %d", uri, resp.StatusCode)
	}
	defer resp.Body.Close()

	// Open the output
	var r io.ReadCloser = resp.Body
	if strings.HasSuffix(uri, ".gz") {
		r, err = gzip.NewReader(r)
		if err != nil {
			return err
		}
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
