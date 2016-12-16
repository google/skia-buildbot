package upload

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/perf/go/ingestcommon"
)

func ObjectPath(benchData *ingestcommon.BenchData, gcsPath string, now time.Time) string {
	keyparts := []string{}
	if benchData.Key != nil {
		for k, v := range benchData.Key {
			keyparts = append(keyparts, k, v)
		}
	}
	filename := fmt.Sprintf("%s_%s.json", benchData.Hash, strings.Join(keyparts, "_"))
	path := filepath.Join(gcsPath, now.Format("2006/01/02/15/"), filename)
	glog.Infof("Writing to path: %q", path)
	return path
}
