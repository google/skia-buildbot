package main

import (
	"flag"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/frontend"
	"go.skia.org/infra/go/auth"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
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

	if err := frontend.UpdateVersionToFuzz(storageClient, *bucket, *versionToFuzz); err != nil {
		fmt.Printf("Problem updating: %s", err)
		os.Exit(1)
	}
}

func setupStorageClient() error {
	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		return fmt.Errorf("Problem setting up client OAuth: %s", err)
	}

	storageClient, err = storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("Problem authenticating: %s", err)
	}
	return nil
}
