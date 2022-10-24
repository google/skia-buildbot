package download

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnzipBodyIntoDirectory_InvalidZipFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	body := []byte("not a valid zip file")
	_, err := unzipBodyIntoDirectory(dir, body)
	require.Error(t, err)
}

func TestUnzipBodyIntoDirectory_NoChromeDriverFoundInTheZip_ReturnsError(t *testing.T) {

	// Create a zip file with two files inside.
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	fw, err := zw.Create("foo.txt")
	require.NoError(t, err)
	_, err = fmt.Fprintf(fw, "foo")
	require.NoError(t, err)

	bw, err := zw.Create("bar.txt")
	require.NoError(t, err)
	_, err = fmt.Fprintf(bw, "bar")
	require.NoError(t, err)

	zw.Close()

	dir := t.TempDir()
	_, err = unzipBodyIntoDirectory(dir, b.Bytes())
	require.Contains(t, err.Error(), "could not find 'chromedriver'")
}

func createValidZipFile(t *testing.T) []byte {
	// Create a zip with a single file.
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	fw, err := zw.Create("myfiles/chromedriver")
	require.NoError(t, err)
	_, err = fmt.Fprintf(fw, "foo")
	require.NoError(t, err)
	zw.Close()
	return b.Bytes()
}

func TestUnzipBodyIntoDirectory_ValidZipFile_NoError(t *testing.T) {

	dir := t.TempDir()
	absFilename, err := unzipBodyIntoDirectory(dir, createValidZipFile(t))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "myfiles", "chromedriver"), absFilename)
	body, err := os.ReadFile(absFilename)
	require.NoError(t, err)
	require.Equal(t, "foo", string(body))
}

func TestDownloadAndUnzipDriver_ServerReturnsNon200StatusCode_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, _, err := DownloadAndUnzipDriver(ts.Client(), func() string {
		return ts.URL
	}, func(version string) string {
		return ts.URL
	})
	require.Error(t, err)
}

func TestDownloadAndUnzipDriver_HappyPath(t *testing.T) {

	zipFile := createValidZipFile(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(zipFile)
		require.NoError(t, err)
	}))
	defer ts.Close()

	// Since getVersionFromURL just returns the contents of the response and
	// then passes it directly to getBodyFromURL as the version parameter, as
	// long as we ignore the version agument to driverURL, which we do below,
	// then a server that just returns the zipfile will always work.
	filename, cleanup, err := DownloadAndUnzipDriver(ts.Client(), func() string {
		return ts.URL
	}, func(version string) string {
		return ts.URL
	})
	require.NoError(t, err)
	require.Contains(t, filename, "myfiles/chromedriver")
	require.NoError(t, cleanup())
}
