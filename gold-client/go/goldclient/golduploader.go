package goldclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/api/option"
)

const (
	// GCS_PREFIX is the expected prefix for a GCS URL.
	GCS_PREFIX = "gs://"
)

// GoldUploader implementations provide functions to upload to GCS.
type GoldUploader interface {
	// copy copies a local file to GCS. If data is provided, those
	// bytes may be used instead of read again from disk.
	// The dst string is assumed to have a gs:// prefix.
	// Currently only uploading from a local file to GCS is supported, that is
	// one cannot use gs://foo/bar as 'fileName'
	UploadBytes(data []byte, fileName, dst string) error

	// UploadJSON serializes the given data to JSON and uploads the result to GCS.
	// An implementation can use tempFileName for temporary storage of JSON data.
	UploadJSON(data interface{}, tempFileName, gcsObjectPath string) error
}

// gsutilUploader implements the GoldUploader interface.
type gsutilUploader struct{}

// gsutilAvailable returns true if the 'gsutil' command could be found on the PATH
func gsutilAvailable() bool {
	_, err := exec.LookPath("gsutil")
	return err == nil
}

// gsUtilUploadJson serializes the given data to JSON and writes the result to the given
// tempFileName, then it copies the file to the given path in GCS. gcsObjPath is assumed
// to have the form: <bucket_name>/path/to/object
func (g *gsutilUploader) UploadJSON(data interface{}, tempFileName, gcsObjPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(tempFileName, jsonBytes, 0644); err != nil {
		return err
	}

	// Upload the written file.
	return g.UploadBytes(nil, tempFileName, prefixGCS(gcsObjPath))
}

// prefixGCS adds the "gs://" prefix to the given GCS path.
func prefixGCS(gcsPath string) string {
	return fmt.Sprintf(GCS_PREFIX+"%s", gcsPath)
}

// gsutilCopy shells out to gsutil to copy the given src to the given target. A path
// starting with "gs://" is assumed to be in GCS.
func (g *gsutilUploader) UploadBytes(data []byte, fileName, dst string) error {
	runCmd := exec.Command("gsutil", "cp", fileName, dst)
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		return skerr.Fmt("Error running gsutil. Got output \n%s\n and error: %s", outBytes, err)
	}
	return nil
}

// httpUploader implements the GoldUploader interface using an authenticated (via an OAuth service
// account) http client.
type httpUploader struct {
	client *gstorage.Client
}

func newHttpUploader(ctx context.Context, httpClient *http.Client) (GoldUploader, error) {
	ret := &httpUploader{}
	var err error
	ret.client, err = gstorage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Fmt("Error instantiating storage client: %s", err)
	}
	return ret, nil
}

func (h *httpUploader) UploadBytes(data []byte, fallbackSrc, dst string) error {
	if len(data) == 0 {
		if strings.HasPrefix(fallbackSrc, GCS_PREFIX) {
			return skerr.Fmt("Copying from a remote file is not supported")
		}

		var err error
		data, err = ioutil.ReadFile(fallbackSrc)
		if err != nil {
			return skerr.Fmt("Error reading file %s: %s", fallbackSrc, err)
		}
	}

	return h.copyBytes(data, dst)
}

func (h *httpUploader) UploadJSON(data interface{}, tempFileName, gcsObjectPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return h.copyBytes(jsonBytes, gcsObjectPath)
}

func (h *httpUploader) copyBytes(data []byte, dst string) error {
	// Trim the prefix and upload the content to the cloud.
	dst = strings.TrimPrefix(dst, GCS_PREFIX)
	bucket, objPath := gcs.SplitGSPath(dst)
	handle := h.client.Bucket(bucket).Object(objPath)

	// TODO(kjlubick): Check if the file exists before-hand and skip uploading unless
	// force is set. This could remove the need to read known_hashes

	w := handle.NewWriter(context.Background())
	_, err := w.Write(data)
	if err != nil {
		_ = w.CloseWithError(err) // Always returns nil, according to docs.
		return err
	}
	return w.Close()
}

// dryRunUploader implements the GoldUploader interface (but doesn't
// actually upload anything)
type dryRunUploader struct{}

func (h *dryRunUploader) UploadBytes(data []byte, fallbackSrc, dst string) error {
	fmt.Printf("dryrun -- upload bytes from %s to %s\n", fallbackSrc, dst)
	return nil
}

func (h *dryRunUploader) UploadJSON(data interface{}, tempFileName, gcsObjectPath string) error {
	fmt.Printf("dryrun -- upload JSON from %s to %s\n", tempFileName, gcsObjectPath)
	return nil
}
