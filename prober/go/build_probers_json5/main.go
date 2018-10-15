// Utility that combines all probers.json5 documents into a single file allprobers.json5.
//
// Exits with an error if there are any name conflicts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/prober/go/types"
)

// flags
var (
	srcdir = flag.String("srcdir", "..", "The directory to start from.")
	dest   = flag.String("dest", "allprobers.json5", "The destination file.")
	depth  = flag.Int("depth", 1, "Depth in subdirectories to search for probers.json5 files.")
)

func main() {
	common.Init()
	// Collect all files named probers.json5.
	files := []string{}
	err := filepath.Walk(*srcdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && len(strings.Split(path, string(filepath.Separator)))-1 > *depth {
			return filepath.SkipDir
		}
		if info.Name() == "probers.json5" {
			sklog.Infof("Found: %q", path)
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		sklog.Fatalf("Failed walking the directory tree at %q: %s", *srcdir, err)
	}

	allProbes := types.Probes{}
	// allProbeSources keeps track of where each probe came from, for error reporting purposes.
	//                 map[probe name]"filename it came from".
	allProbeSources := map[string]string{}
	// Read each collected file, adding elements to allProbes.
	for _, filename := range files {
		if err := add(filename, allProbes, allProbeSources); err != nil {
			sklog.Fatalf("Failed to import probes: %s", err)
		}
	}

	// Serialize to *dest.
	file, err := os.Create(*dest)
	if err != nil {
		sklog.Errorf("Failed to create destination file %q: %s", *dest, err)
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(allProbes); err != nil {
		sklog.Errorf("Failed to write destination file %q: %s", *dest, err)
	}
	util.Close(file)
}

// add the contents of filename to allProbes and allProbeSources.
func add(filename string, allProbes types.Probes, allProbeSources map[string]string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Failed to open file %q: %s", filename, err)
	}
	defer util.Close(file)
	d := json5.NewDecoder(file)
	probes := &types.Probes{}
	if err := d.Decode(probes); err != nil {
		return fmt.Errorf("Failed to decode JSON in file %q: %s", filename, err)
	}
	for k, v := range *probes {
		if _, ok := allProbes[k]; ok {
			return fmt.Errorf("Found duplicate probe name: %q appears in %q and %q", k, filename, allProbeSources[k])
		}
		allProbeSources[k] = filename
		allProbes[k] = v
	}
	return nil
}
