// tiletool is a command line application to validate a tile store.
package main

import (
	"flag"
	"log"
)

import (
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/validator"

	_ "skia.googlesource.com/buildbot.git/golden/go/types" // Registers GoldenTrace with gob so we can read Golden Tiles.
)

// flags
var (
	tileDir    = flag.String("tile_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	verbose    = flag.Bool("verbose", false, "Verbose.")
	echoHashes = flag.Bool("echo_hashes", false, "Echo Git hashes during validation.")
)

func main() {
	flag.Parse()
	store := filetilestore.NewFileTileStore(*tileDir, "nano", 0)
	if !validator.ValidateDataset(store, *verbose, *echoHashes) {
		log.Fatal("FAILED Validation.")
	}
}
