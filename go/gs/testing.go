package gs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
)

const (
	// GS bucket where we store test data. Add a folder to this bucket
	// with the tests for a particular component.
	GS_TEST_DATA_ROOT_URI = "http://storage.googleapis.com/skia-infra-testdata/"
)

func openUri(uriPath string) (*http.Response, error) {
	uri := GS_TEST_DATA_ROOT_URI + uriPath

	client := httputils.NewTimeoutClient()
	request, err := RequestForStorageURL(uri)
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

// DownloadTestDataFile downloads a file with test data from Google Storage.
// The uriPath identifies what to download from the test bucket in GS.
// The content must be publicly accessible.
// The file will be downloaded and stored at provided target
// path (regardless of what the original name is).
// If the the uri ends with '.gz' it will be transparently unzipped.
func DownloadTestDataFile(t assert.TestingT, uriPath, targetPath string) error {
	dir, _ := filepath.Split(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	resp, err := openUri(uriPath)
	if err != nil {
		return err
	}
	defer func() { assert.Nil(t, resp.Body.Close()) }()

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
	defer func() { assert.Nil(t, f.Close()) }()
	_, err = io.Copy(f, r)
	return err
}

// DownloadTestDataArchive downloads testfiles that are stored in
// a gz compressed tar archive and decompresses them into the provided
// target directory.
func DownloadTestDataArchive(t assert.TestingT, uriPath, targetDir string) error {
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
	defer func() { assert.Nil(t, resp.Body.Close()) }()

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

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		} else {
			f, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tarReader)
			if err != nil {
				return err
			}
			defer func() { assert.Nil(t, f.Close()) }()
		}
	}

	return nil
}
