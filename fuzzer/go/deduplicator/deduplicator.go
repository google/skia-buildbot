package deduplicator

import (
	"fmt"
	"sync"

	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/util"
)

const _MAX_STACKTRACE_LINES = 4

type Deduplicator interface {
	Clear()
	IsUnique(report data.FuzzReport) bool
}

// localDeduplicator keeps a local cache of keys based on what has been passed to IsUnique
type localDeduplicator struct {
	seen  map[string]bool
	mutex sync.Mutex
}

func NewLocalDeduplicator() Deduplicator {
	return &localDeduplicator{
		seen: make(map[string]bool),
	}
}

func (d *localDeduplicator) Clear() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.seen = make(map[string]bool)
}

func (d *localDeduplicator) IsUnique(report data.FuzzReport) bool {
	// Empty stacktraces should be manually deduplicated.
	if report.DebugStackTrace.IsEmpty() && report.ReleaseStackTrace.IsEmpty() {
		return true
	}
	// Other flags should also be looked at manually.
	if util.In("Other", report.DebugFlags) || util.In("Other", report.ReleaseFlags) {
		return true
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if k := key(report); d.seen[k] {
		return false
	} else {
		d.seen[k] = true
		return true
	}
}

func key(r data.FuzzReport) string {
	ds := trim(r.DebugStackTrace)
	rs := trim(r.ReleaseStackTrace)
	return fmt.Sprintf("C:%s,A:%s,F:%q,F:%q,S:%s,S:%s", r.FuzzCategory, r.FuzzArchitecture, r.DebugFlags, r.ReleaseFlags, ds.String(), rs.String())
}

// trim returns a copy of the given stacktrace, with the line numbers removed and all but the
// first _MAX_STACKTRACE_LINES stacktraces removed.
func trim(st data.StackTrace) data.StackTrace {
	if frames := st.Frames; len(frames) > _MAX_STACKTRACE_LINES {
		st.Frames = st.Frames[0:_MAX_STACKTRACE_LINES]
	}
	// copy the frames, so we don't accidentally change the real report.
	st.Frames = append([]data.StackTraceFrame(nil), st.Frames...)
	// Remove line numbers from our deduping criteria.
	for i := range st.Frames {
		st.Frames[i].LineNumber = 0
	}
	return st
}
