package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	srcdir = flag.String("srcdir", "..", "The directory to start from.")
	dest   = flag.String("dest", "allskiapush.json5", "The destination file.")
	depth  = flag.Int("depth", 1, "Depth in subdirectories to search for skiapush.json5 files.")
)

func main() {
	common.Init()
	// Collect all files named skiapush.json5.
	files := []string{}
	err := filepath.Walk(*srcdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && len(strings.Split(path, string(filepath.Separator)))-1 > *depth {
			return filepath.SkipDir
		}
		if info.Name() == "skiapush.json5" {
			sklog.Infof("Found: %q", path)
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		sklog.Fatalf("Failed walking the directory tree at %q: %s", *srcdir, err)
	}

	allPush := packages.New()
	// allPushSources keeps track of where each push config file came from, for error reporting purposes.
	//                 map[server name]"filename it came from".
	allPushSources := map[string]string{}
	// Read each collected file, adding elements to allPush and allPushSources.
	for _, filename := range files {
		if err := add(filename, allPush, allPushSources); err != nil {
			sklog.Fatalf("Failed to import config from %q: %s", filename, err)
		}
	}

	// Serialize to *dest.
	file, err := os.Create(*dest)
	if err != nil {
		sklog.Errorf("Failed to create destination file %q: %s", *dest, err)
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(allPush); err != nil {
		sklog.Errorf("Failed to write destination file %q: %s", *dest, err)
	}
	util.Close(file)
}

// add the contents of filename to allPush and allPushSources.
func add(filename string, allPush packages.PackageConfig, allPushSources map[string]string) error {
	config, err := packages.LoadPackageConfig(filename)
	if err != nil {
		sklog.Fatalf("Failed to load PackageConfig file: %s", err)
	}

	for k, v := range config.Servers {
		if _, ok := allPush.Servers[k]; ok {
			return fmt.Errorf("Found duplicate push config name: %q appears in %q and %q", k, filename, allPushSources[k])
		}
		allPushSources[k] = filename
		allPush.Servers[k] = v
	}
	return nil
}
