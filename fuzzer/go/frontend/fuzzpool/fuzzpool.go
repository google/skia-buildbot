package fuzzpool

// The main feature of the fuzzpool package is FuzzPool, a simple in-memory
// cache of FuzzReports that supports simple querying.

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"sync"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/sklog"
)

// FuzzPool has a staging copy of the FuzzReports and a Current copy.
// All queries return values from Current and all writes are to staging.  This
// allows us to fill the staging FuzzPool incrementally and then flip it over
// to Current when everything is loaded w/o being in a partially available state.
type FuzzPool struct {
	staging data.SortedFuzzReports
	// Current is exported so it can be stored to fuzzcache
	Current data.SortedFuzzReports

	mutex sync.Mutex
}

// New returns an empty *FuzzPool.
func New() *FuzzPool {
	return &FuzzPool{
		staging: data.SortedFuzzReports{},
		Current: data.SortedFuzzReports{},
	}
}

// NewForTests is a convenience function for creating a pre-loaded *FuzzPool.
func NewForTests(r []data.FuzzReport) *FuzzPool {
	return &FuzzPool{
		staging: data.SortedFuzzReports{},
		Current: r,
	}
}

// AddFuzzReport adds the given fuzz report to the pool. There is no deduplication, as that
// is assumed to be done before uploading to GCS, which is before reports end up in the pool.
func (p *FuzzPool) AddFuzzReport(r data.FuzzReport) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	r.FileName, r.FunctionName, r.LineNumber = extractStacktraceInfo(r.DebugStackTrace, r.ReleaseStackTrace)
	p.staging = p.staging.Append(r)
}

// ClearStaging empties the staging portion of the pool.
func (p *FuzzPool) ClearStaging() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.staging = data.SortedFuzzReports{}
}

// CurrentFromStaging copies the staging portion of the pool to current.
func (p *FuzzPool) CurrentFromStaging() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.Current = cloneReports(p.staging)
}

// StagingFromCurrent copies the current portion of the pool to staging.
func (p *FuzzPool) StagingFromCurrent() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.staging = cloneReports(p.Current)
}

// Reports returns all FuzzReports
func (p *FuzzPool) Reports() []data.FuzzReport {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.Current
}

// FindFuzzDetailForFuzz returns a FuzzReport of the fuzz with the specified name
// or an error if it could not be found.
func (p *FuzzPool) FindFuzzDetailForFuzz(name string) (data.FuzzReport, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	i := sort.Search(len(p.Current), func(j int) bool {
		return p.Current[j].FuzzName >= name
	})
	if i >= len(p.Current) || p.Current[i].FuzzName != name {
		return data.FuzzReport{}, fmt.Errorf("Fuzz with name %s not found", name)
	}
	return p.Current[i], nil
}

// FindFuzzDetails returns a slice of FuzzReports that match the specified parameters.  "" means
// don't care, except for line, in which common.UNKNOWN_LINE means don't care.  It returns
// an error if there are no matches.
func (p *FuzzPool) FindFuzzDetails(category, architecture, greyOrBad, file, function string, line int) ([]data.FuzzReport, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	retVal := []data.FuzzReport{}
	for _, report := range p.Current {
		if category != "" && category != report.FuzzCategory {
			continue
		}
		if architecture != "" && architecture != report.FuzzArchitecture {
			continue
		}
		if file != "" && file != report.FileName {
			continue
		}
		if function != "" && function != report.FunctionName {
			continue
		}
		if greyOrBad != "" && ((greyOrBad == "grey") != (report.IsGrey)) {
			continue
		}
		if line != common.UNKNOWN_LINE && line != report.LineNumber {
			continue
		}
		retVal = append(retVal, report)
	}
	if len(retVal) == 0 {
		return nil, fmt.Errorf("No fuzzes matched the input critera: C:%s A: %s, badorGrey: %s, file: %s, function: %s, line: %d", category, architecture, greyOrBad, file, function, line)
	}
	return retVal, nil
}

// cloneReport makes a copy of the input using the gob library.
func cloneReports(r data.SortedFuzzReports) data.SortedFuzzReports {
	var temp bytes.Buffer
	enc := gob.NewEncoder(&temp)
	dec := gob.NewDecoder(&temp)

	if err := enc.Encode(r); err != nil {
		// This should never happen, but log it if it does
		sklog.Errorf("Error while cloning report: %v", err)
	}
	var clone data.SortedFuzzReports
	if err := dec.Decode(&clone); err != nil {
		// This should never happen, but log it if it does
		sklog.Errorf("Error while cloning report: %v", err)
	}
	return clone
}

// extractStacktraceInfo returns the file name, function name and line number that
// a report with the given debug and release stacktrace should be sorted by.
// this tries to read the release stacktrace first, falling back to the debug stacktrace,
// failling back to Unknown.
func extractStacktraceInfo(debug, release data.StackTrace) (reportFileName, reportFunctionName string, reportLineNumber int) {
	reportFileName = common.UNKNOWN_FILE
	reportFunctionName = common.UNKNOWN_FUNCTION
	reportLineNumber = common.UNKNOWN_LINE

	stacktrace := release
	if stacktrace.IsEmpty() {
		stacktrace = debug
	}
	if !stacktrace.IsEmpty() {
		frame := stacktrace.Frames[0]
		reportFileName = frame.PackageName + frame.FileName
		reportFunctionName, reportLineNumber = frame.FunctionName, frame.LineNumber
	}
	return
}
