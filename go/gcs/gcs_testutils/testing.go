package gcs_testutils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"google.golang.org/api/option"
)

const (
	// GCS bucket where we store test data. Add a folder to this bucket
	// with the tests for a particular component.
	TEST_DATA_BUCKET = "skia-infra-testdata"
)

func getStorangeItem(bucket, gsPath string) (*storage.Reader, error) {
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(httputils.NewTimeoutClient()))
	if err != nil {
		return nil, err
	}

	return storageClient.Bucket(bucket).Object(gsPath).NewReader(context.Background())
}

// DownloadTestDataFile downloads a file with test data from Google Storage.
// The uriPath identifies what to download from the test bucket in GCS.
// The content must be publicly accessible.
// The file will be downloaded and stored at provided target
// path (regardless of what the original name is).
// If the the uri ends with '.gz' it will be transparently unzipped.
func DownloadTestDataFile(t assert.TestingT, bucket, gsPath, targetPath string) error {
	dir, _ := filepath.Split(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	arch, err := getStorangeItem(bucket, gsPath)
	if err != nil {
		return fmt.Errorf("Could not get gs://%s/%s: %s", bucket, gsPath, err)
	}
	defer func() { assert.NoError(t, arch.Close()) }()

	// Open the output
	var r io.ReadCloser = arch
	if strings.HasSuffix(gsPath, ".gz") {
		r, err = gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("Could not read gzip file: %s", err)
		}
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("Could not create target path: %s", err)
	}
	defer func() { assert.NoError(t, f.Close()) }()
	_, err = io.Copy(f, r)
	return err
}

// DownloadTestDataArchive downloads testfiles that are stored in
// a gz compressed tar archive and decompresses them into the provided
// target directory.
func DownloadTestDataArchive(t assert.TestingT, bucket, gsPath, targetDir string) error {
	if !strings.HasSuffix(gsPath, ".tar.gz") {
		return fmt.Errorf("Expected .tar.gz file. But got:%s", gsPath)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	arch, err := getStorangeItem(bucket, gsPath)
	if err != nil {
		return fmt.Errorf("Could not get gs://%s/%s: %s", bucket, gsPath, err)
	}
	defer func() { assert.NoError(t, arch.Close()) }()

	// Open the output
	r, err := gzip.NewReader(arch)
	if err != nil {
		return fmt.Errorf("Could not read gzip archive: %s", err)
	}
	tarReader := tar.NewReader(r)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("Problem reading from tar archive: %s", err)
		}

		targetPath := filepath.Join(targetDir, hdr.Name)

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("Could not make %s: %s", targetPath, err)
			}
		} else {
			f, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("Could not create target file %s: %s", targetPath, err)
			}
			_, err = io.Copy(f, tarReader)
			if err != nil {
				return fmt.Errorf("Problem while copying: %s", err)
			}
			defer func() { assert.NoError(t, f.Close()) }()
		}
	}

	return nil
}
