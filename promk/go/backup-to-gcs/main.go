package main

import (
	"context"
	"flag"
	"fmt"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"google.golang.org/api/option"
)

// flags
var (
	input  = flag.String("input", "", "Name of file to read.")
	bucket = flag.String("bucket", "", "The name of the GCS bucket to write the backup to.")
	output = flag.String("output", "", "The GCS path where backups should go.")
)

// Backs up the file at the given location, e.g. to a location in GCS. /mnt/grafana/grafana.db
func main() {
	fmt.Println("vim-go")

	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up client OAuth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}
}
