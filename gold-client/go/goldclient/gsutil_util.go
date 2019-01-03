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
	// gsPrefix is the expected prefix for a GCS URL.
	gsPrefix = "gs://"
)

// gsutilUploader implements the cloudUploader interface.
type gsutilUploader struct{}

// gsutilAvailable returns true if the 'gsutil' command could be found on the PATH
func gsutilAvailable() bool {
	_, err := exec.LookPath("gsutil")
	return err == nil
}

// gsUtilUploadJson serializes the given data to JSON and writes the result to the given
// tempFileName, then it copies the file to the given path in GCS. gcsObjPath is assumed
// to have the form: <bucket_name>/path/to/object
func (g gsutilUploader) uploadJson(data interface{}, tempFileName, gcsObjPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(tempFileName, jsonBytes, 0644); err != nil {
		return err
	}

	// Upload the written file.
	return g.uploadBytesOrFile(nil, tempFileName, prefixGCS(gcsObjPath))
}

// prefixGCS adds the "gs://" prefix to the given GCS path.
func prefixGCS(gcsPath string) string {
	return fmt.Sprintf(gsPrefix+"%s", gcsPath)
}

// gsutilCopy shells out to gsutil to copy the given src to the given target. A path
// starting with "gs://" is assumed to be in GCS.
func (g gsutilUploader) uploadBytesOrFile(data []byte, fileName, dst string) error {
	runCmd := exec.Command("gsutil", "cp", fileName, dst)
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		return skerr.Fmt("Error running gsutil. Got output \n%s\n and error: %s", outBytes, err)
	}
	return nil
}

// httpUploader implements the cloudUploader interface using an authenticated (via an OAuth service
// account) http client.
type httpUploader struct {
	client *gstorage.Client
}

func newHttpUploader(ctx context.Context, httpClient *http.Client) (cloudUploader, error) {
	ret := &httpUploader{}
	var err error
	ret.client, err = gstorage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Fmt("Error instantiating storage client: %s", err)
	}
	return ret, nil
}

func (h *httpUploader) uploadBytesOrFile(data []byte, fallbackSrc, dst string) error {
	if len(data) == 0 {
		if strings.HasPrefix(fallbackSrc, gsPrefix) {
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

func (h *httpUploader) uploadJson(data interface{}, tempFileName, gcsObjectPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return h.copyBytes(jsonBytes, gcsObjectPath)
}

func (h *httpUploader) copyBytes(data []byte, dst string) error {
	// Trim the prefix and upload the content to the cloud.
	dst = strings.TrimPrefix(dst, gsPrefix)
	bucket, objPath := gcs.SplitGSPath(dst)
	handle := h.client.Bucket(bucket).Object(objPath)

	w := handle.NewWriter(context.Background())
	_, err := w.Write(data)
	if err != nil {
		_ = w.CloseWithError(err) // Always returns nil, according to docs.
		return err
	}
	return w.Close()
}
