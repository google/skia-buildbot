// Utility that combines all probersk.json5 documents into a single file allprobersk.json5.
//
// Exits with an error if there are any name conflicts.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/proberk/go/types"
)

// flags
var (
	srcdir = flag.String("srcdir", "..", "The directory to start from.")
	dest   = flag.String("dest", "allprobersk.json", "The destination file.")
	depth  = flag.Int("depth", 1, "Depth in subdirectories to search for probers.json5 files.")
)

func main() {
	ctx := context.Background()
	common.Init()
	// Collect all files named probersk.json5.
	files := []string{}
	p, err := filepath.Abs(*srcdir)
	if err != nil {
		sklog.Fatalf("Could not resolve srcdir %s: %s", *srcdir, err)
	}
	sklog.Infof("Traversing %s", p)
	depthOfBuildbotRepoRoot := len(strings.Split(p, string(filepath.Separator)))
	err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		depthOfFile := len(strings.Split(path, string(filepath.Separator)))
		if info.IsDir() && depthOfFile-depthOfBuildbotRepoRoot > *depth {
			return filepath.SkipDir
		}
		if info.Name() == "probersk.json" {
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
		if err := add(ctx, filename, allProbes, allProbeSources); err != nil {
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
func add(ctx context.Context, filename string, allProbes types.Probes, allProbeSources map[string]string) error {
	probes, err := types.LoadFromJSONFile(ctx, filename)
	if err != nil {
		return fmt.Errorf("failed to decode JSON in file %q: %s", filename, err)
	}
	for k, v := range probes {
		if _, ok := allProbes[k]; ok {
			return fmt.Errorf("found duplicate probe name: %q appears in %q and %q", k, filename, allProbeSources[k])
		}
		allProbeSources[k] = filename
		allProbes[k] = v
	}
	return nil
}
