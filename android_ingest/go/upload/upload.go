package upload

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/perf/go/ingestcommon"
)

func ObjectPath(benchData *ingestcommon.BenchData, gcsPath string, now time.Time) string {
	keyparts := []string{}
	for k, v := range benchData.Key {
		keyparts = append(keyparts, k, v)
	}
	filename := fmt.Sprintf("%s_%s.json", benchData.Hash, strings.Join(keyparts, "_"))
	return filepath.Join(gcsPath, now.Format("2006/01/02/15/"), filename)
}
