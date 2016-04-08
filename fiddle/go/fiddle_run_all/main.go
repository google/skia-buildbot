// Command line application that allows executing all the fiddles currently
// stored in Google Storage and then writing back the results of running the
// fiddles into Google Storage.
//
// Presumes fiddle_build has already been run to download and build Skia in the
// right place.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/runner"
	"go.skia.org/infra/fiddle/go/store"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

// flags
var (
	fiddleRoot  = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	gitHash     = flag.String("git_hash", "", "The version of Skia code to run against.")
	resume      = flag.String("resume", "", "The fiddle hash to begin running at.")
	skipSources = flag.Bool("skip_sources", true, "If true then don't download the source images.")
)

func copyDownDrawCpp(st *store.Store, ctx context.Context, hash string) error {
	code, opts, err := st.GetCode(hash)
	if err != nil {
		return fmt.Errorf("Failed to download code to compile: %s", err)
	}
	_, err = runner.WriteDrawCpp(*fiddleRoot, code, opts, true)
	return err
}

func main() {
	// Init.
	common.Init()
	if *fiddleRoot == "" {
		glog.Fatalf("The flag --fiddle_root is required.")
	}
	if *gitHash == "" {
		glog.Fatalf("The flag --git_hash is required.")
	}
	ctx := context.Background()
	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		glog.Fatalf("Problem setting up client OAuth: %s", err)
	}
	st, err := storage.NewClient(ctx, cloud.WithBaseHTTP(client))
	if err != nil {
		glog.Fatalf("Problem creating storage client: %s", err)
	}
	bucket := st.Bucket(store.FIDDLE_STORAGE_BUCKET)
	fiddleStore, err := store.New()
	if err != nil {
		glog.Fatalf("Failed to create fiddle store: %s", err)
	}
	ts, err := runner.GitHashTimeStamp(*fiddleRoot, *gitHash)
	if err != nil {
		glog.Fatalf("Failed to determine gitHash's timestamp: %s", err)
	}

	// Copy down source images.
	if !*skipSources {
		if err := fiddleStore.DownloadAllSourceImages(*fiddleRoot); err != nil {
			glog.Fatalf("Failed to download source images: %s", err)
		}
	}

	// Find the hashes of all the fiddles.
	q := &storage.Query{
		Delimiter: "/",
		Prefix:    fmt.Sprintf("fiddle/"),
	}
	fiddleHashes := []string{}
	for {
		list, err := bucket.List(ctx, q)
		if err != nil {
			glog.Fatalf("Failed to retrieve list: %s", err)
		}
		for _, name := range list.Prefixes {
			fiddleHashes = append(fiddleHashes, strings.Split(name, "/")[1])
		}
		if list.Next == nil {
			break
		}
		q = list.Next
	}
	// Since fiddle_run_all takes 10-12 hours, we have a way to resume from a
	// specific hash.
	if *resume != "" {
		fiddleHashes = fiddleHashes[sort.SearchStrings(fiddleHashes, *resume):]
	}
	glog.Infof("Ececuting %d fiddles.", len(fiddleHashes))

	// Loop over all fiddles, run each then upload the results if successful.
	var fails *os.File
	if *resume != "" {
		fails, err = os.OpenFile("failures.txt", os.O_APPEND|os.O_CREATE, 0644)
	} else {
		fails, err = os.Create("failures.txt")
	}
	if err != nil {
		glog.Fatalf("Failed to create/open failures.txt file: %s", err)
	}
	defer util.Close(fails)

	total := 0
	failures := 0
	for _, fiddleHash := range fiddleHashes {
		total++
		glog.Infof("Current %d/%d failures (%f%%)", failures, total, 100*float32(failures)/float32(total))
		if err := os.Remove(filepath.Join(*fiddleRoot, "src", "draw.cpp")); err != nil {
			glog.Errorf("Failed to delete draw.cpp %s: %s", fiddleHash, err)
		}
		if err := copyDownDrawCpp(fiddleStore, ctx, fiddleHash); err != nil {
			glog.Infof("Failed downloading %s: %s", fiddleHash, err)
			continue
		}

		// Run the fiddle.
		res, err := runner.Run(*fiddleRoot, *gitHash, true)
		if err != nil {
			glog.Errorf("Failed to run fiddle: %s", err)
			continue
		}

		// Log the errors.
		if res.Errors != "" || res.Compile.Errors != "" || res.Execute.Errors != "" {
			fmt.Fprintf(fails, "%s - %q - %q - %q - %q\n", fiddleHash, res.Errors, res.Compile.Errors, res.Compile.Output, res.Execute.Errors)
			failures++
			if err := fails.Sync(); err != nil {
				glog.Errorf("Failed to sync failures.txt: %s", err)
			}
		}

		// Save the results.
		if err := fiddleStore.PutMedia(fiddleHash, *gitHash, ts, res); err != nil {
			glog.Errorf("Failed to write results for run: %s", err)
		}
		glog.Infof("== Write results for %s", fiddleHash)
	}
}
