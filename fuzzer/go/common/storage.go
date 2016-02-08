package common

import (
	"fmt"
	"strings"

	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/gs"
	"google.golang.org/cloud/storage"
)

// GetAllFuzzNamesInFolder returns all the fuzz names in a given GCS folder.  It basically
// returns a list of all files that don't end with a .dump or .err, or error
// if there was a problem.
func GetAllFuzzNamesInFolder(s *storage.Client, name string) (hashes []string, err error) {
	filter := func(item *storage.ObjectAttrs) {
		name := item.Name
		if strings.Contains(name, ".") {
			return
		}
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		hashes = append(hashes, fuzzHash)
	}

	if err = gs.AllFilesInDir(s, config.GS.Bucket, name, filter); err != nil {
		return hashes, fmt.Errorf("Problem getting fuzzes from folder %s: %s", name, err)
	}
	return hashes, nil
}
