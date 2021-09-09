// backup-to-gcs backs up files to GCS on a daily basis, putting the files
// in a directory structure by the day, i.e. /foo/2006/01/02/somefile.db.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

// flags
var (
	bucket   = flag.String("bucket", "", "The name of the GCS bucket to write the backup to.")
	input    = flag.String("input", "", "Name of file to read.")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	output   = flag.String("output", "", "The GCS path where backups should go.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func step(storageClient *storage.Client) error {
	b, err := ioutil.ReadFile(*input)
	if err != nil {
		return fmt.Errorf("Failed to read input file: %s", err)
	}
	fullpath := path.Join(*output, time.Now().Format("2006/01/02"), filepath.Base(*input))
	writer := storageClient.Bucket(*bucket).Object(fullpath).NewWriter(context.Background())
	defer util.Close(writer)
	if _, err = writer.Write(b); err != nil {
		return fmt.Errorf("Failed to write output file: %q: %s", fullpath, err)
	}
	sklog.Infof("Successful backup to %q", fullpath)
	return nil
}

// Backs up the file at the given location, e.g. to a location in GCS. /mnt/grafana/grafana.db
func main() {
	common.InitWithMust("backup-to-gcs", common.PrometheusOpt(promPort))

	ts, err := auth.NewDefaultTokenSource(*local, auth.ScopeFullControl)
	if err != nil {
		sklog.Fatalf("Problem setting up client OAuth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Problem creating storage client: %s", err)
	}
	if err := step(storageClient); err != nil {
		sklog.Fatal(err)
	}

	liveness := metrics2.NewLiveness("backup", map[string]string{"input": *input, "output": *output, "bucket": *bucket})
	for range time.Tick(24 * time.Hour) {
		if err := step(storageClient); err != nil {
			sklog.Error(err)
		}
		liveness.Reset()
	}
}
