package data

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
)

type FuzzReportTree []FileFuzzReport

type FileFuzzReport struct {
	FileName  string               `json:"fileName"`
	Count     int                  `json:"count"`
	Functions []FunctionFuzzReport `json:"byFunction"`
}

type FunctionFuzzReport struct {
	FunctionName string           `json:"functionName"`
	Count        int              `json:"count"`
	LineNumbers  []LineFuzzReport `json:"byLineNumber"`
}

type LineFuzzReport struct {
	LineNumber int               `json:"lineNumber"`
	Count      int               `json:"count"`
	Details    SortedFuzzReports `json:"reports"`
}

type FuzzReport struct {
	DebugStackTrace    StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace  StackTrace `json:"releaseStackTrace"`
	HumanReadableFlags []string   `json:"flags"`

	FuzzName     string `json:"fuzzName"`
	FuzzCategory string `json:"category"`
}

type SortedFuzzReports []FuzzReport

// ParseReport creates a report given the raw materials passed in.
func ParseReport(fuzzName, debugDump, debugErr, releaseDump, releaseErr string) FuzzReport {
	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	return FuzzReport{
		DebugStackTrace:    result.DebugStackTrace,
		ReleaseStackTrace:  result.ReleaseStackTrace,
		HumanReadableFlags: result.Flags.ToHumanReadableFlags(),
		FuzzName:           fuzzName,
		FuzzCategory:       "", // Will be filled in later, when added to the tree
	}
}

// treeReportBuilder is an in-memory structure that allows easy creation of a tree of reports
// for use on the frontend. It has a fuzzReportCache for every fuzz type (e.g. skpicture, skcodec, etc)
type treeReportBuilder struct {
	caches map[string]*fuzzReportCache
	mutex  sync.Mutex
}

// newBuilder creates an initialized treeReportBuilder
func newBuilder() *treeReportBuilder {
	return &treeReportBuilder{
		caches: map[string]*fuzzReportCache{},
	}
}

// A fuzzReportCache holds three FuzzReportTrees - one for the raw data, a sorted version with
// all of the reports and an empty tree that holds no reports.  These are used to procure data
// for the frontend.
type fuzzReportCache struct {
	// All the data goes in here, in no particular order
	rawData FuzzReportTree
	// Generated, sorted caches
	FullReport    FuzzReportTree
	SummaryReport FuzzReportTree

	// If data is in rawData, but not in SummaryReport or FullReport, the trees should be
	// rebuilt
	isDirty bool
}

// currentData is the object that holds the cache of fuzz results.  It is used by the frontend.
var currentData = newBuilder()

// stagingData is the object that processes can write to to queue up new data
// without disturbing the data shown to users.
var stagingData = newBuilder()

func FindFuzzSummary(category string) FuzzReportTree {
	cache, found := currentData.caches[category]
	if !found {
		return FuzzReportTree{}
	}
	return cache.SummaryReport
}

// FindFuzzDetails returns the detailed fuzz reports for a file name, function name, and line number.
// If functionName is "" or lineNumber is -1, all reports are shown.
func FindFuzzDetails(category, fileName, functionName string, lineNumber int) (FileFuzzReport, error) {
	cache := currentData.caches[category]
	for _, file := range cache.FullReport {
		if file.FileName == fileName {
			if functionName == "" {
				return file, nil
			}
			file.filterByFunctionName(functionName)
			if lineNumber == common.UNKNOWN_LINE {
				return file, nil
			}
			file.Functions[0].filterByLineNumber(lineNumber)
			return file, nil
		}
	}
	return FileFuzzReport{}, fmt.Errorf("File %q not found", fileName)
}

// filterByFunctionName removes all FuzzReportFunction except that which matches functionName
func (file *FileFuzzReport) filterByFunctionName(functionName string) {
	for _, function := range file.Functions {
		if functionName == function.FunctionName {
			file.Functions = []FunctionFuzzReport{function}
			break
		}
	}
}

// filterByLineNumber removes all FuzzReportLineNumber except that which matches lineNumber
func (function *FunctionFuzzReport) filterByLineNumber(lineNumber int) {
	for _, line := range function.LineNumbers {
		if lineNumber == line.LineNumber {
			function.LineNumbers = []LineFuzzReport{line}
		}
	}
}

func CategoryOverview(category string) FuzzReportTree {
	overview, found := currentData.caches[category]
	if found {
		return overview.SummaryReport
	}
	return FuzzReportTree{}
}

// FindFuzzDetailForFuzz returns a tree containing the single
// report with the given name, or an error, it it doesn't exist.
func FindFuzzDetailForFuzz(category, name string) (FileFuzzReport, error) {
	if cache, found := currentData.caches[category]; found {
		for _, file := range cache.FullReport {
			if file.filterByFuzzName(name) {
				return file, nil
			}
		}
	}
	return FileFuzzReport{}, fmt.Errorf("Fuzz with name %q not found", name)
}

// filterByFuzzName filters out all functions that do not contain a fuzz with the given
// name and returns true.  If such a fuzz does not exist, it returns false.
func (file *FileFuzzReport) filterByFuzzName(name string) bool {
	for _, function := range file.Functions {
		if function.filterByFuzzName(name) {
			file.Functions = []FunctionFuzzReport{function}
			return true
		}
	}
	return false
}

// filterByFuzzName filters out all lines that do not contain a fuzz with the given
// name and returns true.  If such a fuzz does not exist, it returns false.
func (function *FunctionFuzzReport) filterByFuzzName(name string) bool {
	for _, line := range function.LineNumbers {
		if line.filterByFuzzName(name) {
			function.LineNumbers = []LineFuzzReport{line}
			return true
		}
	}
	return false
}

// filterByFuzzName filters out all fuzzes that do not have the given
// name and returns true.  If such a fuzz does not exist, it returns false.
func (line *LineFuzzReport) filterByFuzzName(name string) bool {
	if b, hasIt := line.Details.containsName(name); hasIt {
		line.Details = SortedFuzzReports{b}
		return true
	}
	return false
}

func NewFuzzFound(category string, b FuzzReport) {
	// set the category if it has not already been set
	b.FuzzCategory = category
	stagingData.addFuzzReport(category, b)
}

// ClearStaging clears the staging representation.
func ClearStaging() {
	stagingData.mutex.Lock()
	defer stagingData.mutex.Unlock()
	stagingData.caches = map[string]*fuzzReportCache{}
}

// SetStaging replaces the staging representation with the given FuzzReport.
func SetStaging(category string, r FuzzReportTree) {
	stagingData.mutex.Lock()
	defer stagingData.mutex.Unlock()
	cache, found := stagingData.caches[category]
	if !found {
		cache = &fuzzReportCache{}
		stagingData.caches[category] = cache
	}
	cache.rawData = r
	cache.rebuildSortedReports()
}

// StagingToCurrent moves a copy of the staging data to the currentData.
func StagingToCurrent() {
	currentData.mutex.Lock()
	defer currentData.mutex.Unlock()
	stagingData.mutex.Lock()
	defer stagingData.mutex.Unlock()

	currentData.caches = map[string]*fuzzReportCache{}
	for k, v := range stagingData.caches {
		cache := fuzzReportCache{}
		cache.rawData = cloneReport(v.rawData)
		cache.rebuildSortedReports()
		currentData.caches[k] = &cache
	}
}

// StagingToCurrent moves a copy of the current data to the staging data.
func StagingFromCurrent() {
	currentData.mutex.Lock()
	defer currentData.mutex.Unlock()
	stagingData.mutex.Lock()
	defer stagingData.mutex.Unlock()

	stagingData.caches = map[string]*fuzzReportCache{}
	for k, v := range currentData.caches {
		cache := fuzzReportCache{}
		cache.rawData = cloneReport(v.rawData)
		cache.rebuildSortedReports()
		stagingData.caches[k] = &cache
	}
}

// StagingCopy returns a fresh copy of the underlying staging data.
func StagingCopy(category string) FuzzReportTree {
	stagingData.mutex.Lock()
	defer stagingData.mutex.Unlock()
	cache, found := stagingData.caches[category]
	if !found {
		return FuzzReportTree{}
	}
	return cloneReport(cache.rawData)
}

// addFuzzReport adds a FuzzReport to a treeReportBuilder's data member
func (r *treeReportBuilder) addFuzzReport(category string, b FuzzReport) {
	reportFileName, reportFunctionName, reportLineNumber := extractStacktraceInfo(b.DebugStackTrace, b.ReleaseStackTrace)

	cache, found := r.caches[category]
	if !found {
		cache = &fuzzReportCache{}
		r.caches[category] = cache
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	foundFile, foundFunction, foundLine := cache.makeOrFindRecords(reportFileName, reportFunctionName, reportLineNumber)

	foundFile.Count++
	foundFunction.Count++
	foundLine.Count++
	foundLine.Details = foundLine.Details.append(b)
	cache.isDirty = true

}

// extractStacktraceInfo returns the file name, function name and line number that
// a report with the given debug and release stacktrace should be sorted by.
// this tries to read the release stacktrace first, falling back to the debug stacktrace,
// failling back to Unknown.
func extractStacktraceInfo(debug, release StackTrace) (reportFileName, reportFunctionName string, reportLineNumber int) {
	reportFileName, reportFunctionName, reportLineNumber = common.UNKNOWN_FILE, common.UNKNOWN_FUNCTION, common.UNKNOWN_LINE

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

// makeOrFindRecords finds the FuzzReportFile, FuzzReportFunction and FuzzReportLineNumber
// associated with the inputs, creating the structures if needed.
func (c *fuzzReportCache) makeOrFindRecords(reportFileName, reportFunctionName string, reportLineNumber int) (*FileFuzzReport, *FunctionFuzzReport, *LineFuzzReport) {
	var foundFile *FileFuzzReport
	for i, file := range c.rawData {
		if file.FileName == reportFileName {
			foundFile = &c.rawData[i]
			break
		}
	}
	if foundFile == nil {
		c.rawData = append(c.rawData, FileFuzzReport{reportFileName, 0, nil})
		foundFile = &c.rawData[len(c.rawData)-1]
	}

	var foundFunction *FunctionFuzzReport
	for i, function := range foundFile.Functions {
		if function.FunctionName == reportFunctionName {
			foundFunction = &foundFile.Functions[i]
			break
		}
	}
	if foundFunction == nil {
		foundFile.Functions = append(foundFile.Functions, FunctionFuzzReport{reportFunctionName, 0, nil})
		foundFunction = &foundFile.Functions[len(foundFile.Functions)-1]
	}

	var foundLine *LineFuzzReport
	for i, line := range foundFunction.LineNumbers {
		if line.LineNumber == reportLineNumber {
			foundLine = &foundFunction.LineNumbers[i]
		}
	}
	if foundLine == nil {
		foundFunction.LineNumbers = append(foundFunction.LineNumbers, LineFuzzReport{reportLineNumber, 0, nil})
		foundLine = &foundFunction.LineNumbers[len(foundFunction.LineNumbers)-1]
	}
	return foundFile, foundFunction, foundLine
}

// getTreeSortedByTotal gets the detailed FuzzReport for a fuzz category
// sorted by total number of fuzzes.
func (r *treeReportBuilder) getTreeSortedByTotal(category string) FuzzReportTree {
	cache, found := r.caches[category]
	if !found {
		glog.Warningf("Could not find report tree for category %s", category)
		return FuzzReportTree{}
	}
	if cache.isDirty {
		r.mutex.Lock()
		defer r.mutex.Unlock()
		cache.rebuildSortedReports()
	}
	return cache.FullReport
}

// getSummarySortedByTotal gets the summary FuzzReport for a fuzz category
// sorted by total number of fuzzes.
func (r *treeReportBuilder) getSummarySortedByTotal(category string) FuzzReportTree {
	cache, found := r.caches[category]
	if !found {
		glog.Warningf("Could not find report tree for category %s", category)
		return FuzzReportTree{}
	}
	if cache.isDirty {
		r.mutex.Lock()
		defer r.mutex.Unlock()
		cache.rebuildSortedReports()
	}
	return cache.SummaryReport
}

// rebuildSortedReports creates the sorted reports for a given cache.
func (c *fuzzReportCache) rebuildSortedReports() {
	c.FullReport = c.getClonedSortedReport(true)
	c.SummaryReport = c.getClonedSortedReport(false)
	c.isDirty = false
}

// getClonedSortedReport makes a newly allocated FuzzReport after running the passed in function
// on all FuzzReportLineNumber objects in the report.
func (c *fuzzReportCache) getClonedSortedReport(keepDetails bool) FuzzReportTree {
	report := cloneReport(c.rawData)
	sort.Sort(filesTotalSort(report))
	for i := range report {
		file := &report[i]
		sort.Sort(functionsTotalSort(file.Functions))
		for j := range file.Functions {
			function := &file.Functions[j]
			sort.Sort(linesTotalSort(function.LineNumbers))
			for k := range function.LineNumbers {
				line := &function.LineNumbers[k]
				if !keepDetails {
					line.Details = nil
				}
			}
		}
	}
	return report
}

// cloneReport makes a copy of the input using the gob library.
func cloneReport(data []FileFuzzReport) FuzzReportTree {
	var temp bytes.Buffer
	enc := gob.NewEncoder(&temp)
	dec := gob.NewDecoder(&temp)

	if err := enc.Encode(data); err != nil {
		// This should never happen, but log it if it does
		glog.Errorf("Error while cloning report: %v", err)
	}
	var clone FuzzReportTree
	if err := dec.Decode(&clone); err != nil {
		// This should never happen, but log it if it does
		glog.Errorf("Error while cloning report: %v", err)
	}
	return clone
}

// Total sort methods - sorts files, functions and lines by Count
type filesTotalSort []FileFuzzReport

func (r filesTotalSort) Len() int      { return len(r) }
func (r filesTotalSort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

func (r filesTotalSort) Less(i, j int) bool {
	if r[i].Count != r[j].Count {
		return r[i].Count > r[j].Count
	}
	// If they have the same total, sort by name
	return r[i].FileName < r[j].FileName
}

type functionsTotalSort []FunctionFuzzReport

func (r functionsTotalSort) Len() int      { return len(r) }
func (r functionsTotalSort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

func (r functionsTotalSort) Less(i, j int) bool {
	if r[i].Count != r[j].Count {
		return r[i].Count > r[j].Count
	}
	// If they have the same total, sort by name
	return r[i].FunctionName < r[j].FunctionName
}

type linesTotalSort []LineFuzzReport

func (r linesTotalSort) Len() int      { return len(r) }
func (r linesTotalSort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

func (r linesTotalSort) Less(i, j int) bool {
	if r[i].Count != r[j].Count {
		return r[i].Count > r[j].Count
	}
	// If they have the same total, sort by line number
	return r[i].LineNumber < r[j].LineNumber
}

func (p SortedFuzzReports) Len() int           { return len(p) }
func (p SortedFuzzReports) Less(i, j int) bool { return p[i].FuzzName < p[j].FuzzName }
func (p SortedFuzzReports) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// append adds b to the already sorted caller, and returns the sorted result.
// Precondition: Caller must be nil or sorted
func (p SortedFuzzReports) append(b FuzzReport) SortedFuzzReports {
	s := append(p, b)

	// Google Storage gives us the fuzzes in alphabetical order.  Thus, we can short circuit
	// if the fuzz goes on the end (which is usually does).
	// However, we can't always do this because when we load a second batch of fuzzes,
	// those are in alphabetical order, but starting over from 0.
	// We want to avoid [a,c,x,z,b,d] where b,d were added from the second batch.
	if len(s) <= 1 || s.Less(len(s)-2, len(s)-1) {
		return s
	}
	sort.Sort(s)
	return s
}

// containsName returns the FuzzReport and true if a fuzz with the given name is in the list.
func (p SortedFuzzReports) containsName(fuzzName string) (FuzzReport, bool) {
	i := sort.Search(len(p), func(i int) bool { return p[i].FuzzName >= fuzzName })
	if i < len(p) && p[i].FuzzName == fuzzName {
		return p[i], true
	}
	return FuzzReport{}, false
}
