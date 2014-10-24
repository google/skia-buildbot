// tiletool is a command line application to validate a tile store.
package main

import (
	"flag"
	"fmt"

        "github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/flags"
	"skia.googlesource.com/buildbot.git/perf/go/validator"
)

// Command line flags.
var (
	tileDir    = flag.String("tile_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	verbose    = flag.Bool("verbose", false, "Verbose.")
	echoHashes = flag.Bool("echo_hashes", false, "Echo Git hashes during validation.")
	dataset    = flag.String("dataset", config.DATASET_NANO, fmt.Sprintf("Choose from the valid datasets: %v", config.VALID_DATASETS))
)

func main() {
	flag.Parse()
	flags.Log()
	if !util.In(*dataset, config.VALID_DATASETS) {
		glog.Fatalf("Not a valid dataset: %s", *dataset)
	}
	store := filetilestore.NewFileTileStore(*tileDir, *dataset, 0)
	if !validator.ValidateDataset(store, *verbose, *echoHashes) {
		glog.Fatal("FAILED Validation.")
	}
}
