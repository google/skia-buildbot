package common

// TODO(kjlubick): Move this to package storage, where possible/reasonable.
import (
	"fmt"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/gcs"
)

// ExtractFuzzNameFromPath turns a path name into a fuzz name by stripping off all but the
// last piece from the path.
func ExtractFuzzNameFromPath(path string) (name string) {
	return path[strings.LastIndex(path, "/")+1:]
}

// ExtractFuzzNamesFromPaths turns all path names into just fuzz names, by extracting the
// last piece of the path.
func ExtractFuzzNamesFromPaths(paths []string) (names []string) {
	names = make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, ExtractFuzzNameFromPath(path))
	}
	return names
}

// GetAllFuzzNamesInFolder returns all the fuzz names in a given GCS folder.  It basically
// returns a list of all files that don't end with a .dump or .err, or error
// if there was a problem.
func GetAllFuzzNamesInFolder(s *storage.Client, name string) (hashes []string, err error) {
	filter := func(item *storage.ObjectAttrs) {
		name := item.Name
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if !IsNameOfFuzz(fuzzHash) {
			return
		}
		hashes = append(hashes, fuzzHash)
	}

	if err = gcs.AllFilesInDir(s, config.GCS.Bucket, name, filter); err != nil {
		return hashes, fmt.Errorf("Problem getting fuzzes from folder %s: %s", name, err)
	}
	return hashes, nil
}

// IsNameOfFuzz returns true if the GCS file name given is a fuzz, which is basically if it doesn't
// have a . in it.
func IsNameOfFuzz(name string) bool {
	return name != "" && !strings.Contains(name, ".")
}
