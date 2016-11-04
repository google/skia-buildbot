package dataframe

import (
	"time"

	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/ptracestore"

	"github.com/skia-dev/glog"
)

var vcs vcsinfo.VCS

// StartWarmer runs a query that extends over the last year of data to keep the
// tiles warm in the disk cache.
func StartWarmer(v vcsinfo.VCS) {
	vcs = v
	go warmer()
}

func warmer() {
	onestep()
	for _ = range time.Tick(time.Hour) {
		onestep()
	}
}

func onestep() {
	defer timer.New("Warmer onestep").Stop()
	end := time.Now()
	begin := time.Now().Add(-365 * 24 * time.Hour)
	colHeaders, commitIDs, skip := getRange(vcs, begin, end)
	matches := func(key string) bool {
		return false
	}
	_, err := _new(colHeaders, commitIDs, matches, ptracestore.Default, nil, skip)
	if err != nil {
		glog.Errorf("Failed building the dataframe while warming.", err)
	}

}
