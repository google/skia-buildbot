package perfclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/perf/go/ingestcommon"
)

type PerfClient interface {
	PushToPerf(now time.Time, folderName, filePrefix string, data ingestcommon.BenchData) error
}

type perfClient struct {
	storageClient gcs.GCSClient
	basePath      string
}

func New(basePath string, s gcs.GCSClient) PerfClient {
	return &perfClient{
		storageClient: s,
		basePath:      basePath,
	}
}

func (pb *perfClient) PushToPerf(now time.Time, folderName, filePrefix string, data ingestcommon.BenchData) error {

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Error converting to json: %s", err)
	}

	compressed := bytes.Buffer{}
	cw := gzip.NewWriter(&compressed)
	if c, err := cw.Write(b); err != nil {
		return fmt.Errorf("Could not compress json object, only wrote %d bytes: %s", c, err)
	}
	if err = cw.Close(); err != nil {
		return fmt.Errorf("Could not finish compressing json object: %s", err)
	}

	path := objectPath(&data, pb.basePath, folderName, filePrefix, now, b)

	opts := gcs.FileWriteOptions{
		// If we compress with gzip and then upload to GCS, GCS allows for auto
		// uncompression. See https://cloud.google.com/storage/docs/transcoding
		ContentEncoding: "gzip",
		ContentType:     "application/json",
	}

	return pb.storageClient.SetFileContents(context.Background(), path, opts, compressed.Bytes())
}

// ObjectPath returns the Google Cloud Storage path where the JSON serialization
// of benchData should be stored.
//
// gcsPath will be the root of the path.
// now is the time which will be encoded in the path.
// b is the source bytes of the incoming file.
func objectPath(benchData *ingestcommon.BenchData, gcsPath, folderName, filePrefix string, now time.Time, b []byte) string {
	hash := fmt.Sprintf("%x", md5.Sum(b))
	keyparts := []string{}
	if benchData.Key != nil {
		for k, v := range benchData.Key {
			keyparts = append(keyparts, k, v)
		}
	}
	filename := fmt.Sprintf("%s_%s_%d.json", filePrefix, hash, now.UnixNano()/int64(time.Millisecond))
	return filepath.Join(gcsPath, now.Format("2006/01/02/15/"), folderName, filename)
}
