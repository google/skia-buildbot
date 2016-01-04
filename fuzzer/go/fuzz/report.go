package fuzz

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
	FileName    string               `json:"fileName"`
	BinaryCount int                  `json:"binaryCount"`
	ApiCount    int                  `json:"apiCount"`
	Functions   []FunctionFuzzReport `json:"byFunction"`
}

type FunctionFuzzReport struct {
	FunctionName string           `json:"functionName"`
	BinaryCount  int              `json:"binaryCount"`
	ApiCount     int              `json:"apiCount"`
	LineNumbers  []LineFuzzReport `json:"byLineNumber"`
}

type LineFuzzReport struct {
	LineNumber    int                     `json:"lineNumber"`
	BinaryCount   int                     `json:"binaryCount"`
	ApiCount      int                     `json:"apiCount"`
	BinaryDetails SortedBinaryFuzzReports `json:"binaryReports"`
	APIDetails    SortedAPIFuzzReports    `json:"apiReports"`
}

type BinaryFuzzReport struct {
	DebugStackTrace    StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace  StackTrace `json:"releaseStackTrace"`
	HumanReadableFlags []string   `json:"flags"`

	BadBinaryName string `json:"binaryName"`
	BinaryType    string `json:"binaryType"`
}

type APIFuzzReport struct {
	DebugStackTrace    StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace  StackTrace `json:"releaseStackTrace"`
	HumanReadableFlags []string   `json:"flags"`

	TestName string `json:"testName"`
}

type SortedBinaryFuzzReports []BinaryFuzzReport
type SortedAPIFuzzReports []APIFuzzReport

// ParseBinaryReport creates a binary report given the raw materials passed in.
func ParseBinaryReport(fuzzType, fuzzName, debugDump, debugErr, releaseDump, releaseErr string) BinaryFuzzReport {
	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	return BinaryFuzzReport{result.DebugStackTrace, result.ReleaseStackTrace, result.Flags.ToHumanReadableFlags(), fuzzName, fuzzType}
}

// treeReportBuilder is an in-memory structure that allows easy creation of a tree of reports
// for use on the frontend.  It caches three separate trees, one containing just the binary
// reports, just the api reports and no reports (binaryReport, apiReport and summaryReport,
// respectively.
type treeReportBuilder struct {
	dataMutex sync.Mutex
	data      []FileFuzzReport // The raw data

	summaryReport FuzzReportTree // The created, cached report roots
	binaryReport  FuzzReportTree
	apiReport     FuzzReportTree

	summaryDirty bool
	binaryDirty  bool
	apiDirty     bool
}

// currentData is the object that holds the cache of fuzz results.  It is used by the frontend.
var currentData treeReportBuilder

// stagingData is the object that processes can write to to queue up new data
// without disturbing the data shown to users.
var stagingData treeReportBuilder

// FuzzSummary returns the summary of all fuzzes, sorted by total count.
func FuzzSummary() FuzzReportTree {
	return currentData.getSummarySortedByTotal()
}

// FindFuzzDetails returns the detailed fuzz reports for a file name, function name, and line number.
// If functionName is "" or lineNumber is -1, all reports are shown.
func FindFuzzDetails(fileName, functionName string, lineNumber int, useBinary bool) (FileFuzzReport, error) {

	var report FuzzReportTree
	if useBinary {
		report = currentData.getBinaryTreeSortedByTotal()
	} else {
		report = currentData.getAPITreeSortedByTotal()
	}

	for _, file := range report {
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

// FindFuzzDetailForFuzz returns a tree containing the single binary or api
// report with the given name, or an error, it it doesn't exist.
func FindFuzzDetailForFuzz(name string) (FileFuzzReport, error) {
	// Look in binary first
	report := currentData.getBinaryTreeSortedByTotal()
	for _, file := range report {
		if file.filterByFuzzName(name) {
			return file, nil
		}
	}
	report = currentData.getAPITreeSortedByTotal()
	for _, file := range report {
		if file.filterByFuzzName(name) {
			return file, nil
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
	if b, hasIt := line.BinaryDetails.containsName(name); hasIt {
		line.BinaryDetails = SortedBinaryFuzzReports{b}
		line.APIDetails = SortedAPIFuzzReports{}
		return true
	}
	if a, hasIt := line.APIDetails.containsName(name); hasIt {
		line.BinaryDetails = SortedBinaryFuzzReports{}
		line.APIDetails = SortedAPIFuzzReports{a}
		return true
	}
	return false
}

// NewBinaryFuzzFound adds a FuzzReportBinary to the in-memory staging representation.
func NewBinaryFuzzFound(b BinaryFuzzReport) {
	stagingData.addReportBinary(b)
}

// NewAPIFuzzFound adds a FuzzReportAPI to the in-memory staging representation.
func NewAPIFuzzFound(a APIFuzzReport) {
	stagingData.addReportAPI(a)
}

// ClearStaging clears the staging representation.
func ClearStaging() {
	SetStaging([]FileFuzzReport{})
}

// SetStaging replaces the staging representation with the given FuzzReport.
func SetStaging(r FuzzReportTree) {
	stagingData.dataMutex.Lock()
	defer stagingData.dataMutex.Unlock()
	stagingData.data = r
	stagingData.markDirty()
}

// StagingToCurrent moves a copy of the staging data to the currentData.
func StagingToCurrent() {
	currentData.dataMutex.Lock()
	defer currentData.dataMutex.Unlock()
	stagingData.dataMutex.Lock()
	defer stagingData.dataMutex.Unlock()
	currentData.data = cloneReport(stagingData.data)
	currentData.markDirty()
}

// StagingCopy returns a fully copy of the underlying staging data.
func StagingCopy() FuzzReportTree {
	stagingData.dataMutex.Lock()
	defer stagingData.dataMutex.Unlock()
	return cloneReport(stagingData.data)
}

// addReportBinary adds a FuzzReportBinary to a treeReportBuilder's data member
func (r *treeReportBuilder) addReportBinary(b BinaryFuzzReport) {
	reportFileName, reportFunctionName, reportLineNumber := extractStacktraceInfo(b.DebugStackTrace, b.ReleaseStackTrace)

	r.dataMutex.Lock()
	foundFile, foundFunction, foundLine := r.makeOrFindRecords(reportFileName, reportFunctionName, reportLineNumber)

	foundFile.BinaryCount++
	foundFunction.BinaryCount++
	foundLine.BinaryCount++
	foundLine.BinaryDetails = foundLine.BinaryDetails.append(b)
	r.markDirty()
	r.dataMutex.Unlock()
}

// addReportBinary adds a FuzzReportAPI to a treeReportBuilder's data member
func (r *treeReportBuilder) addReportAPI(a APIFuzzReport) {
	reportFileName, reportFunctionName, reportLineNumber := extractStacktraceInfo(a.DebugStackTrace, a.ReleaseStackTrace)

	r.dataMutex.Lock()
	foundFile, foundFunction, foundLine := r.makeOrFindRecords(reportFileName, reportFunctionName, reportLineNumber)

	foundFile.ApiCount++
	foundFunction.ApiCount++
	foundLine.ApiCount++
	foundLine.APIDetails = foundLine.APIDetails.append(a)
	r.markDirty()
	r.dataMutex.Unlock()
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
func (r *treeReportBuilder) makeOrFindRecords(reportFileName, reportFunctionName string, reportLineNumber int) (*FileFuzzReport, *FunctionFuzzReport, *LineFuzzReport) {
	var foundFile *FileFuzzReport
	for i, file := range r.data {
		if file.FileName == reportFileName {
			foundFile = &r.data[i]
			break
		}
	}
	if foundFile == nil {
		r.data = append(r.data, FileFuzzReport{reportFileName, 0, 0, nil})
		foundFile = &r.data[len(r.data)-1]
	}

	var foundFunction *FunctionFuzzReport
	for i, function := range foundFile.Functions {
		if function.FunctionName == reportFunctionName {
			foundFunction = &foundFile.Functions[i]
			break
		}
	}
	if foundFunction == nil {
		foundFile.Functions = append(foundFile.Functions, FunctionFuzzReport{reportFunctionName, 0, 0, nil})
		foundFunction = &foundFile.Functions[len(foundFile.Functions)-1]
	}

	var foundLine *LineFuzzReport
	for i, line := range foundFunction.LineNumbers {
		if line.LineNumber == reportLineNumber {
			foundLine = &foundFunction.LineNumbers[i]
		}
	}
	if foundLine == nil {
		foundFunction.LineNumbers = append(foundFunction.LineNumbers, LineFuzzReport{reportLineNumber, 0, 0, nil, nil})
		foundLine = &foundFunction.LineNumbers[len(foundFunction.LineNumbers)-1]
	}
	return foundFile, foundFunction, foundLine
}

// markDirty marks all the cached components of the builder as dirty, such that they
// should be recreated on next query.
func (r *treeReportBuilder) markDirty() {
	r.summaryDirty = true
	r.binaryDirty = true
	r.apiDirty = true
}

// getBinaryTreeSortedByTotal gets the detailed binary FuzzReport sorted by total number of fuzzes.
func (r *treeReportBuilder) getBinaryTreeSortedByTotal() FuzzReportTree {
	if r.binaryDirty {
		r.binaryReport = r.getClonedSortedReport(func(line *LineFuzzReport) {
			line.APIDetails = nil
		})
		r.binaryDirty = false
	}
	return r.binaryReport
}

// getAPITreeSortedByTotal gets the detailed API FuzzReport sorted by total number of fuzzes.
func (r *treeReportBuilder) getAPITreeSortedByTotal() FuzzReportTree {
	if r.apiDirty {
		r.apiReport = r.getClonedSortedReport(func(line *LineFuzzReport) {
			line.BinaryDetails = nil
		})
		r.apiDirty = false
	}
	return r.apiReport
}

// getSummarySortedByTotal gets the summary FuzzReport sorted by total number of fuzzes.
func (r *treeReportBuilder) getSummarySortedByTotal() FuzzReportTree {
	if r.summaryDirty {
		r.summaryReport = r.getClonedSortedReport(func(line *LineFuzzReport) {
			line.BinaryDetails = nil
			line.APIDetails = nil
		})
		r.summaryDirty = false
	}
	return r.summaryReport
}

// getClonedSortedReport makes a newly allocated FuzzReport after running the passed in function
// on all FuzzReportLineNumber objects in the report.
func (r *treeReportBuilder) getClonedSortedReport(sanitize func(*LineFuzzReport)) FuzzReportTree {
	r.dataMutex.Lock()
	report := cloneReport(r.data)
	r.dataMutex.Unlock()
	sort.Sort(filesTotalSort(report))
	for i := range report {
		file := &report[i]
		sort.Sort(functionsTotalSort(file.Functions))
		for j := range file.Functions {
			function := &file.Functions[j]
			sort.Sort(linesTotalSort(function.LineNumbers))
			for k := range function.LineNumbers {
				line := &function.LineNumbers[k]
				sanitize(line)
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

// Total sort methods - sorts files, functions and lines by APICount + BinaryCount
type filesTotalSort []FileFuzzReport

func (r filesTotalSort) Len() int      { return len(r) }
func (r filesTotalSort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

func (r filesTotalSort) Less(i, j int) bool {
	aTotal := r[i].BinaryCount + r[i].ApiCount
	bTotal := r[j].BinaryCount + r[j].ApiCount
	if aTotal != bTotal {
		return aTotal > bTotal
	}
	// If they have the same total, sort by name
	return r[i].FileName < r[j].FileName
}

type functionsTotalSort []FunctionFuzzReport

func (r functionsTotalSort) Len() int      { return len(r) }
func (r functionsTotalSort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

func (r functionsTotalSort) Less(i, j int) bool {
	aTotal := r[i].BinaryCount + r[i].ApiCount
	bTotal := r[j].BinaryCount + r[j].ApiCount
	if aTotal != bTotal {
		return aTotal > bTotal
	}
	// If they have the same total, sort by name
	return r[i].FunctionName < r[j].FunctionName
}

type linesTotalSort []LineFuzzReport

func (r linesTotalSort) Len() int      { return len(r) }
func (r linesTotalSort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

func (r linesTotalSort) Less(i, j int) bool {
	aTotal := r[i].BinaryCount + r[i].ApiCount
	bTotal := r[j].BinaryCount + r[j].ApiCount
	if aTotal != bTotal {
		return aTotal > bTotal
	}
	// If they have the same total, sort by line number
	return r[i].LineNumber < r[j].LineNumber
}

func (p SortedBinaryFuzzReports) Len() int           { return len(p) }
func (p SortedBinaryFuzzReports) Less(i, j int) bool { return p[i].BadBinaryName < p[j].BadBinaryName }
func (p SortedBinaryFuzzReports) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// append adds b to the already sorted caller, and returns the sorted result.
// Precondition: Caller must be nil or sorted
func (p SortedBinaryFuzzReports) append(b BinaryFuzzReport) SortedBinaryFuzzReports {
	s := append(p, b)

	// Google Storage gives us the fuzzes in alphabetical order.  Thus, we can short circuit
	// if the fuzz goes on the end (which is usually does).
	// However, we can't always do this because when we load a second batch of fuzzes,
	// those are in alphabetical order, but starting over from 0.
	// We want to avoid a,c,x,z,b,d where b,d were added from the second batch.
	if len(s) <= 1 || s.Less(len(s)-2, len(s)-1) {
		return s
	}
	sort.Sort(s)
	return s
}

// containsName returns the Binary report and true if a fuzz with the given name is in the list.
func (p SortedBinaryFuzzReports) containsName(fuzzName string) (BinaryFuzzReport, bool) {
	i := sort.Search(len(p), func(i int) bool { return p[i].BadBinaryName >= fuzzName })
	if i < len(p) && p[i].BadBinaryName == fuzzName {
		return p[i], true
	}
	return BinaryFuzzReport{}, false
}

func (p SortedAPIFuzzReports) Len() int           { return len(p) }
func (p SortedAPIFuzzReports) Less(i, j int) bool { return p[i].TestName < p[j].TestName }
func (p SortedAPIFuzzReports) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// append adds b to the already sorted caller, and returns the sorted result.
// Precondition: Caller must be nil or sorted
func (p SortedAPIFuzzReports) append(a APIFuzzReport) SortedAPIFuzzReports {
	s := append(p, a)

	// See comment on SortedBinaryFuzzReports.append for rationale about the short circuit here.
	if len(s) <= 1 || s.Less(len(s)-2, len(s)-1) {
		return s
	}
	sort.Sort(s)
	return s
}

// containsName returns the API report and true if a fuzz with the given name is in the list.
func (p SortedAPIFuzzReports) containsName(fuzzName string) (APIFuzzReport, bool) {
	i := sort.Search(len(p), func(i int) bool { return p[i].TestName >= fuzzName })
	if i < len(p) && p[i].TestName == fuzzName {
		return p[i], true
	}
	return APIFuzzReport{}, false
}
