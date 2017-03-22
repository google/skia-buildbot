package upload

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/perf/go/ingestcommon"
)

// ObjectPath returns the Google Cloud Storage path where the JSON serialization
// of benchData should be stored.
//
// gcsPath will be the root of the path.
// now is the time which will be encoded in the path.
func ObjectPath(benchData *ingestcommon.BenchData, gcsPath string, now time.Time) string {
	keyparts := []string{}
	if benchData.Key != nil {
		for k, v := range benchData.Key {
			keyparts = append(keyparts, k, v)
		}
	}
	filename := fmt.Sprintf("%s_%s_%d.json", benchData.Hash, strings.Join(keyparts, "_"), now.UnixNano())
	return filepath.Join(gcsPath, now.Format("2006/01/02/15/"), filename)
}
