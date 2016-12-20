package main

// rpi-backup is an executable that backs up a referenced disk image to Google storage.
// It is meant to be run on a timer, e.g. daily.

import (
	"compress/gzip"
	"flag"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

var (
	serviceAccountPath = flag.String("service_account_path", "", "Path to the service account.  Can be empty string to use defaults or project metadata")
	gceBucket          = flag.String("gce_bucket", "skia-images", "GCS Bucket images should be stored in")
	gceFolder          = flag.String("gce_folder", "Swarming", "Folder in the bucket that should hold the images")
	imgPath            = flag.String("img_path", "", "Where the image is stored on disk")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

func main() {
	defer common.LogPanic()
	common.InitExternalWithMetrics2("rpi-backup", influxHost, influxUser, influxPassword, influxDatabase)
	defer metrics2.Flush()
	if *imgPath == "" {
		sklog.Fatalf("You must specify a local image location")
	}

	// We use the plain old http Transport, because the default one doesn't like uploading big files.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountPath, &http.Transport{Dial: httputils.DialTimeout}, auth.SCOPE_READ_WRITE, sklog.CLOUD_LOGGING_WRITE_SCOPE)
	if err != nil {
		sklog.Fatalf("Could not setup credentials: %s", err)
	}

	common.StartCloudLoggingWithClient(client, "skolo-rpi-master", "rpi-backup")

	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Could not authenticate to GCS: %s", err)
	}

	contents, hash, err := fileutil.ReadAndSha1File(*imgPath)
	if err != nil {
		sklog.Fatalf("Could not read image file: %s", err)
	}

	// We name the file using date and sha1 hash of the image
	day := time.Now().Format("2006-01-02")
	name := fmt.Sprintf("%s/%s-%s.gz", *gceFolder, day, hash)
	w := storageClient.Bucket(*gceBucket).Object(name).NewWriter(context.Background())
	defer util.Close(w)

	w.ObjectAttrs.ContentEncoding = "application/gzip"

	gw := gzip.NewWriter(w)
	defer util.Close(gw)

	sklog.Infof("Uploading to gs://%s/%s", *gceBucket, name)

	// This takes a few minutes for a ~1.3 GB image (which gets compressed to about 400MB)
	if i, err := gw.Write([]byte(contents)); err != nil {
		sklog.Fatalf("Problem writing to GCS.  Only wrote %d/%d bytes: %s", i, len(contents), err)
	} else {
		metrics2.GetInt64Metric("skolo.rpi-backup.backup-size", nil).Update(int64(i))
	}

	sklog.Infof("Upload complete")
}
