package download

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"go.skia.org/infra/comp-ui/go/compui/urls"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Cleanup function to be called to clean up files.
type Cleanup func() error

// NoopCleanup is a do-nothing Cleanup function.
var NoopCleanup Cleanup = func() error {
	return nil
}

// DownloadAndUnzipDriver uses the supplied client to download and extract a zip
// file to a temp directory, returning the absolute path the executable in the
// zip file, a Cleanup function to be called to clean up that directory, and
// potentially an error.
func DownloadAndUnzipDriver(client *http.Client, latestURL func() string, driverURL func(version string) string) (string, Cleanup, error) {
	url := latestURL()
	version, err := urls.GetVersionFromURL(url, client)
	if err != nil {
		return "", nil, skerr.Wrapf(err, "Failed to load: %q", url)
	}
	url = driverURL(version)
	body, err := urls.GetBodyFromURL(url, client)
	if err != nil {
		return "", nil, skerr.Wrapf(err, "Failed to load: %q", url)
	}
	tempDir, err := os.MkdirTemp("", "comp-ui-download")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		return os.RemoveAll(tempDir)
	}
	ret, err := unzipBodyIntoDirectory(tempDir, body)
	if err != nil {
		if err := cleanup(); err != nil {
			sklog.Error(err)
		}
		return "", nil, err
	}

	return ret, cleanup, nil
}

// Returns the absolute path to the file.
func unzipBodyIntoDirectory(dir string, body []byte) (string, error) {
	reader := bytes.NewReader(body)
	// Open a zip archive for reading.
	r, err := zip.NewReader(reader, int64(len(body)))
	if err != nil {
		return "", err
	}

	// Find the first filename that has a Base of "chromedriver".
	var filename = ""
	var allFilenames = make([]string, len(r.File))
	for i, file := range r.File {
		allFilenames[i] = file.Name
		if filepath.Base(file.Name) == "chromedriver" {
			filename = file.Name
			break
		}
	}

	if filename == "" {
		return "", fmt.Errorf("could not find 'chromedriver' file in archive: %q", allFilenames)
	}

	f := r.File[0]
	outputFilename := filepath.Join(dir, filepath.FromSlash(f.Name))
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer util.Close(rc)
	unzippedBody, err := ioutil.ReadAll(rc)
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(filepath.Dir(outputFilename), 0755)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outputFilename, unzippedBody, 0755); err != nil {
		return "", err
	}
	return outputFilename, nil
}
