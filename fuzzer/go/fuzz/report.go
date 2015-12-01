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

type FuzzReport []FuzzReportFile

type FuzzReportFile struct {
	FileName    string               `json:"fileName"`
	BinaryCount int                  `json:"binaryCount"`
	ApiCount    int                  `json:"apiCount"`
	Functions   []FuzzReportFunction `json:"byFunction"`
}

type FuzzReportFunction struct {
	FunctionName string                 `json:"functionName"`
	BinaryCount  int                    `json:"binaryCount"`
	ApiCount     int                    `json:"apiCount"`
	LineNumbers  []FuzzReportLineNumber `json:"byLineNumber"`
}

type FuzzReportLineNumber struct {
	LineNumber    int                `json:"lineNumber"`
	BinaryCount   int                `json:"binaryCount"`
	ApiCount      int                `json:"apiCount"`
	BinaryDetails []FuzzReportBinary `json:"binaryReports"`
	APIDetails    []FuzzReportAPI    `json:"apiReports"`
}

type FuzzReportBinary struct {
	DebugStackTrace    StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace  StackTrace `json:"releaseStackTrace"`
	HumanReadableFlags []string   `json:"flags"`

	BadBinaryName string `json:"binaryName"`
	BinaryType    string `json:"binaryType"`
}

type FuzzReportAPI struct {
	DebugStackTrace    StackTrace `json:"debugStackTrace"`
	ReleaseStackTrace  StackTrace `json:"releaseStackTrace"`
	HumanReadableFlags []string   `json:"flags"`

	TestName string `json:"testName"`
}

// ParseBinaryReport creates a binary report given the raw materials passed in.
func ParseBinaryReport(fuzzType, fuzzName, debugDump, debugErr, releaseDump, releaseErr string) FuzzReportBinary {
	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	return FuzzReportBinary{result.DebugStackTrace, result.ReleaseStackTrace, result.Flags.ToHumanReadableFlags(), fuzzName, fuzzType}
}

// fuzzReportBuilder is an in-memory structure that allows easy creation of a tree of reports
// for use on the frontend.  It caches three separate trees, one containing just the binary
// reports, just the api reports and no reports (binaryReport, apiReport and summaryReport,
// respectively.
type fuzzReportBuilder struct {
	dataMutex sync.Mutex
	data      []FuzzReportFile // The raw data

	summaryReport FuzzReport // The created, cached report roots
	binaryReport  FuzzReport
	apiReport     FuzzReport

	summaryDirty bool
	binaryDirty  bool
	apiDirty     bool
}

// reportData is the object that holds the cache of fuzz results.  It is used by the frontend.
var reportData fuzzReportBuilder

// FuzzSummary returns the summary of all fuzzes, sorted by total count.
func FuzzSummary() FuzzReport {
	return reportData.getSummarySortedByTotal()
}

// FuzzDetails returns the detailed fuzz reports for a file name, function name, and line number.
// If functionName is "" or lineNumber is -1, all reports are shown.
func FuzzDetails(fileName, functionName string, lineNumber int, useBinary bool) (FuzzReportFile, error) {

	var report FuzzReport
	if useBinary {
		report = reportData.getBinaryReportSortedByTotal()
	} else {
		report = reportData.getAPIReportSortedByTotal()
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
	return FuzzReportFile{}, fmt.Errorf("File %s not found", fileName)
}

// filterByFunctionName removes all FuzzReportFunction except that which matches functionName
func (file *FuzzReportFile) filterByFunctionName(functionName string) {
	for _, function := range file.Functions {
		if functionName == function.FunctionName {
			file.Functions = []FuzzReportFunction{function}
			break
		}
	}
}

// filterByLineNumber removes all FuzzReportLineNumber except that which matches lineNumber
func (function *FuzzReportFunction) filterByLineNumber(lineNumber int) {
	for _, line := range function.LineNumbers {
		if lineNumber == line.LineNumber {
			function.LineNumbers = []FuzzReportLineNumber{line}
		}
	}
}

// NewBinaryFuzzFound adds a FuzzReportBinary to the in-memory representation.
func NewBinaryFuzzFound(b FuzzReportBinary) {
	reportData.addReportBinary(b)
}

// NewBinaryFuzzFound adds a FuzzReportAPI to the in-memory representation.
func NewAPIFuzzFound(a FuzzReportAPI) {
	reportData.addReportAPI(a)
}

// addReportBinary adds a FuzzReportBinary to a fuzzReportBuilder's data member
func (r *fuzzReportBuilder) addReportBinary(b FuzzReportBinary) {
	reportFileName, reportFunctionName, reportLineNumber := extractStacktraceInfo(b.DebugStackTrace, b.ReleaseStackTrace)

	r.dataMutex.Lock()
	foundFile, foundFunction, foundLine := r.makeOrFindRecords(reportFileName, reportFunctionName, reportLineNumber)

	foundFile.BinaryCount++
	foundFunction.BinaryCount++
	foundLine.BinaryCount++
	foundLine.BinaryDetails = append(foundLine.BinaryDetails, b)
	r.markDirty()
	r.dataMutex.Unlock()
}

// addReportBinary adds a FuzzReportAPI to a fuzzReportBuilder's data member
func (r *fuzzReportBuilder) addReportAPI(a FuzzReportAPI) {
	reportFileName, reportFunctionName, reportLineNumber := extractStacktraceInfo(a.DebugStackTrace, a.ReleaseStackTrace)

	r.dataMutex.Lock()
	foundFile, foundFunction, foundLine := r.makeOrFindRecords(reportFileName, reportFunctionName, reportLineNumber)

	foundFile.ApiCount++
	foundFunction.ApiCount++
	foundLine.ApiCount++
	foundLine.APIDetails = append(foundLine.APIDetails, a)
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
func (r *fuzzReportBuilder) makeOrFindRecords(reportFileName, reportFunctionName string, reportLineNumber int) (*FuzzReportFile, *FuzzReportFunction, *FuzzReportLineNumber) {
	var foundFile *FuzzReportFile
	for i, file := range r.data {
		if file.FileName == reportFileName {
			foundFile = &r.data[i]
			break
		}
	}
	if foundFile == nil {
		r.data = append(r.data, FuzzReportFile{reportFileName, 0, 0, nil})
		foundFile = &r.data[len(r.data)-1]
	}

	var foundFunction *FuzzReportFunction
	for i, function := range foundFile.Functions {
		if function.FunctionName == reportFunctionName {
			foundFunction = &foundFile.Functions[i]
			break
		}
	}
	if foundFunction == nil {
		foundFile.Functions = append(foundFile.Functions, FuzzReportFunction{reportFunctionName, 0, 0, nil})
		foundFunction = &foundFile.Functions[len(foundFile.Functions)-1]
	}

	var foundLine *FuzzReportLineNumber
	for i, line := range foundFunction.LineNumbers {
		if line.LineNumber == reportLineNumber {
			foundLine = &foundFunction.LineNumbers[i]
		}
	}
	if foundLine == nil {
		foundFunction.LineNumbers = append(foundFunction.LineNumbers, FuzzReportLineNumber{reportLineNumber, 0, 0, nil, nil})
		foundLine = &foundFunction.LineNumbers[len(foundFunction.LineNumbers)-1]
	}
	return foundFile, foundFunction, foundLine
}

// markDirty marks all the cached components of the builder as dirty, such that they
// should be recreated on next query.
func (r *fuzzReportBuilder) markDirty() {
	r.summaryDirty = true
	r.binaryDirty = true
	r.apiDirty = true
}

// getBinaryReportSortedByTotal gets the detailed binary FuzzReport sorted by total number of fuzzes.
func (r *fuzzReportBuilder) getBinaryReportSortedByTotal() FuzzReport {
	if r.binaryDirty {
		r.binaryReport = r.getClonedSortedReport(func(line *FuzzReportLineNumber) {
			line.APIDetails = nil
		})
		r.binaryDirty = false
	}
	return r.binaryReport
}

// getAPIReportSortedByTotal gets the detailed API FuzzReport sorted by total number of fuzzes.
func (r *fuzzReportBuilder) getAPIReportSortedByTotal() FuzzReport {
	if r.apiDirty {
		r.apiReport = r.getClonedSortedReport(func(line *FuzzReportLineNumber) {
			line.BinaryDetails = nil
		})
		r.apiDirty = false
	}
	return r.apiReport
}

// getAPIReportSortedByTotal gets the summary FuzzReport sorted by total number of fuzzes.
func (r *fuzzReportBuilder) getSummarySortedByTotal() FuzzReport {
	if r.summaryDirty {
		r.summaryReport = r.getClonedSortedReport(func(line *FuzzReportLineNumber) {
			line.BinaryDetails = nil
			line.APIDetails = nil
		})
		r.summaryDirty = false
	}
	return r.summaryReport
}

// getClonedSortedReport makes a newly allocated FuzzReport after running the passed in function
// on all FuzzReportLineNumber objects in the report.
func (r *fuzzReportBuilder) getClonedSortedReport(sanitize func(*FuzzReportLineNumber)) FuzzReport {
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
func cloneReport(data []FuzzReportFile) FuzzReport {
	var temp bytes.Buffer
	enc := gob.NewEncoder(&temp)
	dec := gob.NewDecoder(&temp)

	if err := enc.Encode(data); err != nil {
		// This should never happen, but log it if it does
		glog.Errorf("Error while cloning report: %v", err)
	}
	var clone FuzzReport
	if err := dec.Decode(&clone); err != nil {
		// This should never happen, but log it if it does
		glog.Errorf("Error while cloning report: %v", err)
	}
	return clone
}

// Total sort methods - sorts files, functions and lines by APICount + BinaryCount
type filesTotalSort []FuzzReportFile

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

type functionsTotalSort []FuzzReportFunction

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

type linesTotalSort []FuzzReportLineNumber

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
