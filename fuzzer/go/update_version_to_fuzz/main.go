package main

import (
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/auth"
	"golang.org/x/net/context"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

var (
	versionToFuzz = flag.String("version_to_fuzz", "", "The full hash of the Skia commit to be set to fuzzed.")
	bucket        = flag.String("bucket", "skia-fuzzer", "The GCS bucket in which to locate found fuzzes.")

	storageClient *storage.Client = nil
)

func main() {
	flag.Parse()
	if err := setupStorageClient(); err != nil {
		fmt.Printf("Could not setup link to GCS: %s\n", err)
		os.Exit(1)
	}

	if *versionToFuzz == "" {
		fmt.Println("--version_to_fuzz cannot be empty")
		os.Exit(1)
	}

	newVersionFile := fmt.Sprintf("skia_version/pending/%s", *versionToFuzz)
	w := storageClient.Bucket(*bucket).Object(newVersionFile).NewWriter(context.Background())
	if err := w.Close(); err != nil {
		fmt.Printf("Could not create version file %s : %s", newVersionFile, err)
		os.Exit(1)
	}
	fmt.Printf("%s has been made.  The backend and frontend will eventually pick up this change (in that order).\n", newVersionFile)
}

func setupStorageClient() error {
	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		return fmt.Errorf("Problem setting up client OAuth: %s", err)
	}

	storageClient, err = storage.NewClient(context.Background(), cloud.WithBaseHTTP(client))
	if err != nil {
		return fmt.Errorf("Problem authenticating: %s", err)
	}
	return nil
}
