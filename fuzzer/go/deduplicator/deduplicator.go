package deduplicator

import (
	"fmt"
	"sync"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/util"
)

const _MAX_STACKTRACE_LINES = 4

type Deduplicator interface {
	// Deduplicators are based on the Skia revision being analyzed. To clear out a deduplicator,
	// set the commit to something else.
	SetRevision(string)
	IsUnique(report data.FuzzReport) bool
}

// localDeduplicator keeps a local cache of keys based on what has been passed to IsUnique
type localDeduplicator struct {
	// maps keys to true/false
	seen  map[string]bool
	mutex sync.Mutex
}

func NewLocalDeduplicator() Deduplicator {
	return &localDeduplicator{
		seen: make(map[string]bool),
	}
}

func (d *localDeduplicator) SetRevision(r string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.seen = make(map[string]bool)
}

func (d *localDeduplicator) IsUnique(report data.FuzzReport) bool {
	allEmpty := true
	for _, st := range report.Stacktraces {
		allEmpty = allEmpty && st.IsEmpty()
	}
	// Empty stacktraces should be manually deduplicated.
	if allEmpty {
		return true
	}

	anyOther := false
	for _, flags := range report.Flags {
		anyOther = anyOther || util.In("Other", flags)
	}
	// Other flags should also be looked at manually.
	if anyOther {
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
	s := fmt.Sprintf("C:%s,A:%s", r.FuzzCategory, r.FuzzArchitecture)
	for _, c := range common.ANALYSIS_TYPES {
		st := trim(r.Stacktraces[c])
		s += fmt.Sprintf("F:%q,S:%s", r.Flags[c], st.String())
	}
	return s
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
