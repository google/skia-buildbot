package main

import (
	"encoding/json"
	"io"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
)

func main() {
	for name, instanceConfig := range config.PERF_BIGTABLE_CONFIGS {
		err := util.WithWriteFile(filepath.Join("configs", name+".json"), func(w io.Writer) error {
			return json.NewEncoder(w).Encode(instanceConfig)
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}
}
