package goldclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	gstorage "cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// gcsPrefix is the expected prefix for a GCS URL.
	gcsPrefix = "gs://"
)

// GCSUploader implementations provide functions to upload to GCS.
type GCSUploader interface {
	// UploadBytes copies a local file to GCS. If data is provided, those
	// bytes may be used instead of read again from disk.
	// The dst string is assumed to have a gs:// prefix.
	// Currently only uploading from a local file to GCS is supported, that is
	// one cannot use gs://foo/bar as 'fileName'
	UploadBytes(data []byte, fileName, dst string) error

	// UploadJSON serializes the given data to JSON and uploads the result to GCS.
	// An implementation can use tempFileName for temporary storage of JSON data.
	UploadJSON(data interface{}, tempFileName, gcsObjectPath string) error
}

type GCSDownloader interface {
	// Download returns the bytes belonging to a GCS file
	Download(ctx context.Context, gcsFile string) ([]byte, error)
}

// gsutilImpl implements the GCSUploader interface.
type gsutilImpl struct{}

// UploadJSON serializes the given data to JSON and writes the result to the given
// tempFileName, then it copies the file to the given path in GCS. gcsObjPath is assumed
// to have the form: <bucket_name>/path/to/object
func (g *gsutilImpl) UploadJSON(data interface{}, tempFileName, gcsObjPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return skerr.Wrapf(err, "could not marshal to JSON before uploading")
	}

	if err := ioutil.WriteFile(tempFileName, jsonBytes, 0644); err != nil {
		return skerr.Wrapf(err, "saving json to %s", tempFileName)
	}

	// Upload the written file.
	return g.UploadBytes(nil, tempFileName, prefixGCS(gcsObjPath))
}

// prefixGCS adds the "gs://" prefix to the given GCS path.
func prefixGCS(gcsPath string) string {
	return fmt.Sprintf(gcsPrefix+"%s", gcsPath)
}

// UploadBytes shells out to gsutil to copy the given src to the given target. A path
// starting with "gs://" is assumed to be in GCS.
func (g *gsutilImpl) UploadBytes(data []byte, fileName, dst string) error {
	runCmd := exec.Command("gsutil", "cp", fileName, dst)
	outBytes, err := runCmd.CombinedOutput()
	if err != nil {
		if runtime.GOOS == "windows" {
			runCmd = exec.Command("python", "gsutil.py", "cp", fileName, dst)
			outBytes, err = runCmd.CombinedOutput()
			if err != nil {
				return skerr.Wrapf(err, "running gsutil. Got output \n%s\n", outBytes)
			}
		} else {
			return skerr.Wrapf(err, "running gsutil. Got output \n%s\n", outBytes)
		}
	}
	return nil
}

// clientImpl implements the GCSUploader interface using an authenticated (via an OAuth service
// account) http client.
type clientImpl struct {
	client *gstorage.Client
}

func newHttpUploader(ctx context.Context, httpClient *http.Client) (GCSUploader, error) {
	ret := &clientImpl{}
	var err error
	ret.client, err = gstorage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Wrapf(err, "instantiating storage client")
	}
	return ret, nil
}

func (h *clientImpl) UploadBytes(data []byte, fallbackSrc, dst string) error {
	if len(data) == 0 {
		if strings.HasPrefix(fallbackSrc, gcsPrefix) {
			return skerr.Fmt("Copying from a remote file is not supported")
		}

		var err error
		data, err = ioutil.ReadFile(fallbackSrc)
		if err != nil {
			return skerr.Wrapf(err, "reading file %s", fallbackSrc)
		}
	}

	return h.copyBytes(data, dst)
}

func (h *clientImpl) UploadJSON(data interface{}, tempFileName, gcsObjectPath string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return skerr.Wrap(err)
	}
	return h.copyBytes(jsonBytes, gcsObjectPath)
}

func (h *clientImpl) copyBytes(data []byte, dst string) error {
	// Trim the prefix and upload the content to the cloud.
	dst = strings.TrimPrefix(dst, gcsPrefix)
	bucket, objPath := gcs.SplitGSPath(dst)
	handle := h.client.Bucket(bucket).Object(objPath)

	// TODO(kjlubick): Check if the file exists before-hand and skip uploading unless
	// force is set. This could remove the need to read known_hashes

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel() // The docs say to cancel the context in the event of an error or success.
	w := handle.NewWriter(ctx)
	_, err := w.Write(data)
	if err != nil {
		return skerr.Wrap(err)
	}
	return w.Close()
}

func (h *clientImpl) Download(ctx context.Context, gcsFile string) ([]byte, error) {
	src := strings.TrimPrefix(gcsFile, gcsPrefix)
	bucket, objPath := gcs.SplitGSPath(src)
	handle := h.client.Bucket(bucket).Object(objPath)

	r, err := handle.NewReader(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting reader for %s", gcsFile)
	}
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "reading from GCS for %s", gcsFile)
	}
	return b, nil
}

// dryRunImpl implements the GCSUploader interface (but doesn't
// actually upload anything)
type dryRunImpl struct{}

func (h *dryRunImpl) UploadBytes(data []byte, fallbackSrc, dst string) error {
	fmt.Printf("dryrun -- upload bytes from %s to %s\n", fallbackSrc, dst)
	return nil
}

func (h *dryRunImpl) UploadJSON(data interface{}, tempFileName, gcsObjectPath string) error {
	fmt.Printf("dryrun -- upload JSON from %s to %s\n", tempFileName, gcsObjectPath)
	return nil
}
