// Convenience utilities for testing.
package testutils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/juju/testing/checkers"

	"skia.googlesource.com/buildbot.git/go/gs"
	"skia.googlesource.com/buildbot.git/go/util"
)

const (
	// GS bucket where we store test data. Add a folder to this bucket
	// with the tests for a particular component.
	GS_TEST_DATA_ROOT_URI = "http://storage.googleapis.com/skia-infra-testdata/"
)

// SkipIfShort causes the test to be skipped when running with -short.
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test with -short")
	}
}

// AssertDeepEqual fails the test if the two objects do not pass reflect.DeepEqual.
func AssertDeepEqual(t *testing.T, a, b interface{}) {
	if eq, err := checkers.DeepEqual(a, b); !eq {
		t.Fatal(err)
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

// DownloadTestDataFile downloads a file with test data from Google Storage.
// The uriPath identifies what to download from the test bucket in GS.
// The content must be publicly accessible.
// The file will be downloaded and stored at provided target
// path (regardless of what the original name is).
// If the the uri ends with '.gz' it will be transparently unzipped.
func DownloadTestDataFile(uriPath, targetPath string) error {
	dir, _ := filepath.Split(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	resp, err := openUri(uriPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Open the output
	var r io.ReadCloser = resp.Body
	if strings.HasSuffix(uriPath, ".gz") {
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

// DownloadTestDataArchive downloads testfiles that are stored in
// a gz compressed tar archive and decompresses them into the provided
// target directory.
func DownloadTestDataArchive(uriPath, targetDir string) error {
	if !strings.HasSuffix(uriPath, ".tar.gz") {
		return fmt.Errorf("Expected .tar.gz file. But got:%s", uriPath)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	resp, err := openUri(uriPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Open the output
	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(r)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		targetPath := filepath.Join(targetDir, hdr.Name)
		f, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, tarReader)
		if err != nil {
			return err
		}
		f.Close()
	}

	return nil
}

func openUri(uriPath string) (*http.Response, error) {
	uri := GS_TEST_DATA_ROOT_URI + uriPath

	client := util.NewTimeoutClient()
	request, err := gs.RequestForStorageURL(uri)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Downloading %s failed. Got response status: %d", uri, resp.StatusCode)
	}

	return resp, nil
}
